import React, { useState, useEffect, useRef } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Dimensions,
  ActivityIndicator,
  Alert,
  Modal,
  StatusBar,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router, useLocalSearchParams } from 'expo-router';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { VideoView, useVideoPlayer } from 'expo-video';
import Slider from '@react-native-community/slider';
import BackHeader from '@/components/BackHeader';
import { useAuth } from '@/contexts/AuthContext';
import axios from 'axios';
import { API_BASE_URL } from '@/config/api';

// Types based on backend video struct
type VideoData = {
  ID: string;
  Title: string;
  Description: string;
  Status: 'PENDING' | 'PROCESSING' | 'COMPLETED' | 'FAILED';
  CreatedAt: string;
  UpdatedAt: string;
  UserID: string;
  ViewCount: number;
  FilePath: string;
  HLSPath: string;
  ThumbnailPath: string;
  Metadata: {
    Duration: number;
    Width: number;
    Height: number;
    Codec: string;
    AudioCodec: string;
    Bitrate: number;
    FrameRate: number;
    FileSize: number;
  };
  Error?: string;
};

const { width: screenWidth } = Dimensions.get('window');
const videoAspectRatio = 16 / 9;
const videoHeight = screenWidth / videoAspectRatio;

export default function VideoScreen() {
  const { logout } = useAuth();
  const { id } = useLocalSearchParams<{ id: string }>();
  
  const [video, setVideo] = useState<VideoData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [showControls, setShowControls] = useState(true);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [quality, setQuality] = useState('auto');
  const [showQualitySelector, setShowQualitySelector] = useState(false);
  const [buffering, setBuffering] = useState(false);
  
  const controlsTimeoutRef = useRef<number | null>(null);

  // Initialize video player
  const player = useVideoPlayer('', player => {
    player.loop = false;
  });

  // Update player source when video data is loaded
  useEffect(() => {
    if (video?.Status === 'COMPLETED' && video?.HLSPath && id) {
      const streamUrl = `${API_BASE_URL}/stream/${id}/playlist.m3u8`;
      
      console.log('🎥 [VIDEO] Loading video stream:', streamUrl);
      console.log('🎥 [VIDEO] Video HLS Path:', video.HLSPath);
      
      // Load the video in the player - the player will handle fetching the HLS playlist
      (async () => {
        try {
          console.log('🎬 [VIDEO] Loading stream in expo-video player...');
          await player.replaceAsync({uri: streamUrl});
          console.log('✅ [VIDEO] Player loaded successfully');
          
        } catch (error: any) {
          console.error('❌ [VIDEO] HLS stream error:', error);
          console.error('❌ [VIDEO] Error details:', {
            name: error.name,
            message: error.message,
            stack: error.stack?.substring(0, 500)
          });
        }
      })();
    } else {
      console.log('⚠️ [VIDEO] Cannot load video:', {
        status: video?.Status,
        hasHLSPath: !!video?.HLSPath,
        hasId: !!id,
        hlsPath: video?.HLSPath
      });
    }
  }, [video, id, player]);

  useEffect(() => {
    console.log('🚀 [VIDEO] useEffect triggered with ID:', id);
    if (id) {
      console.log('📱 [VIDEO] ID exists, calling loadVideo');
      loadVideo();
    } else {
      console.log('⚠️ [VIDEO] No ID provided');
    }
  }, [id]);

  // Listen to player events
  useEffect(() => {
    if (!player) return;

    console.log('🎥 [VIDEO] Player initialized:', player);

    const subscription = player.addListener('playingChange', (event) => {
      console.log('📺 [VIDEO] Playing state changed:', event.isPlaying);
      setIsPlaying(event.isPlaying);
    });

    const timeSubscription = player.addListener('timeUpdate', (event) => {
      setCurrentTime(event.currentTime);
      // Get duration from player directly
      if (player.duration) {
        setDuration(player.duration);
      }
    });

    const statusSubscription = player.addListener('statusChange', (event) => {
      console.log('📺 [VIDEO] Status changed:', event.status);
      setBuffering(event.status === 'loading');
      
      if (event.status === 'error') {
        console.error('❌ [VIDEO] Player error detected, error:', event.error);
      }
    });

    return () => {
      subscription?.remove();
      timeSubscription?.remove();
      statusSubscription?.remove();
    };
  }, [player]);

  useEffect(() => {
    // Hide controls after 3 seconds of inactivity
    if (showControls && isPlaying) {
      controlsTimeoutRef.current = setTimeout(() => {
        setShowControls(false);
      }, 3000);
    }
    
    return () => {
      if (controlsTimeoutRef.current) {
        clearTimeout(controlsTimeoutRef.current);
      }
    };
  }, [showControls, isPlaying]);

  const loadVideo = async () => {
    console.log('🎬 [VIDEO] Starting loadVideo for ID:', id);
    try {
      const token = await AsyncStorage.getItem('userToken');
      console.log('🔑 [VIDEO] Retrieved token:', token ? 'Present' : 'Missing');
      
      if (!token) {
        console.log('❌ [VIDEO] No token found, redirecting to login');
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      console.log('📡 [VIDEO] Making API request to:', `${API_BASE_URL}/api/video/${id}`);
      const response = await axios.get(`${API_BASE_URL}/api/video/${id}`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      console.log('✅ [VIDEO] API Response received:', {
        status: response.status,
        data: response.data
      });
      
      console.log('📊 [VIDEO] Video metadata:', {
        ID: response.data.ID,
        Title: response.data.Title,
        Status: response.data.Status,
        HLSPath: response.data.HLSPath, // Check both cases
        hlsPath: response.data.hlsPath,
        Duration: response.data.Metadata?.Duration,
        Error: response.data.Error
      });

      setVideo(response.data);
      setDuration(response.data.Metadata.Duration);
      
      console.log('💾 [VIDEO] Video state updated successfully');
    } catch (error: any) {
      console.error('❌ [VIDEO] Error loading video:', error);
      console.log('🔍 [VIDEO] Error details:', {
        message: error.message,
        status: error.response?.status,
        data: error.response?.data,
        config: {
          url: error.config?.url,
          method: error.config?.method,
          headers: error.config?.headers
        }
      });
      
      const errorMessage = error.response?.data?.error || 'Failed to load video. Please try again.';
      Alert.alert('Error', errorMessage);
    } finally {
      console.log('🏁 [VIDEO] loadVideo completed, setting isLoading to false');
      setIsLoading(false);
    }
  };

  const handlePlayPause = async () => {
    console.log('▶️ [VIDEO] handlePlayPause called, current state:', { isPlaying });
    if (player) {
      try {
        if (isPlaying) {
          console.log('⏸️ [VIDEO] Pausing video');
          player.pause();
        } else {
          console.log('▶️ [VIDEO] Playing video');
          player.play();
        }
        setIsPlaying(!isPlaying);
        console.log('✅ [VIDEO] Play/pause successful, new state:', !isPlaying);
      } catch (error) {
        console.error('❌ [VIDEO] Error in handlePlayPause:', error);
      }
    } else {
      console.log('⚠️ [VIDEO] player is null');
    }
  };

  const handleSeek = async (time: number) => {
    console.log('⏩ [VIDEO] handleSeek called with time:', time);
    if (player && video) {
      try {
        console.log('🎯 [VIDEO] Setting video position to:', time, 'seconds');
        player.currentTime = time;
        setCurrentTime(time);
        console.log('✅ [VIDEO] Seek successful');
        
        // Update backend with seek time for analytics
        try {
          const token = await AsyncStorage.getItem('userToken');
          if (token) {
            console.log('📡 [VIDEO] Updating timestamp on backend:', time);
            await axios.get(`${API_BASE_URL}/api/video/${id}/timestamp?current=${time}`, {
              headers: { 'Authorization': `Bearer ${token}` },
            });
            console.log('✅ [VIDEO] Timestamp updated on backend');
          }
        } catch (error) {
          console.log('⚠️ [VIDEO] Failed to update timestamp:', error);
        }
      } catch (error) {
        console.error('❌ [VIDEO] Error in handleSeek:', error);
      }
    } else {
      console.log('⚠️ [VIDEO] Cannot seek - player or video missing');
    }
  };

  const handleVideoPress = () => {
    console.log('👆 [VIDEO] Video pressed, toggling controls. Current state:', showControls);
    setShowControls(!showControls);
    console.log('🎛️ [VIDEO] Controls will be:', !showControls ? 'shown' : 'hidden');
  };

  const handleFullscreen = () => {
    setIsFullscreen(!isFullscreen);
    StatusBar.setHidden(!isFullscreen);
  };

  const formatTime = (seconds: number): string => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const formatViewCount = (count: number): string => {
    if (count >= 1000000) {
      return `${(count / 1000000).toFixed(1)}M views`;
    } else if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}K views`;
    }
    return `${count} views`;
  };

  const formatFileSize = (bytes: number): string => {
    const MB = bytes / (1024 * 1024);
    const GB = MB / 1024;
    return GB >= 1 ? `${GB.toFixed(1)} GB` : `${MB.toFixed(1)} MB`;
  };

  const getVideoStatus = (status: string) => {
    switch (status) {
      case 'PENDING':
        return { text: 'Processing...', color: '#FF9500' };
      case 'PROCESSING':
        return { text: 'Transcoding...', color: '#007AFF' };
      case 'COMPLETED':
        return { text: 'Ready', color: '#34C759' };
      case 'FAILED':
        return { text: 'Failed', color: '#FF3B30' };
      default:
        return { text: 'Unknown', color: '#8E8E93' };
    }
  };

  if (isLoading) {
    console.log('⏳ [VIDEO] Rendering loading state');
    return (
      <SafeAreaView style={styles.container}>
        <BackHeader title="Loading..." />
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color="#007AFF" />
          <Text style={styles.loadingText}>Loading video...</Text>
        </View>
      </SafeAreaView>
    );
  }

  if (!video) {
    console.log('❌ [VIDEO] Rendering error state - no video data');
    return (
      <SafeAreaView style={styles.container}>
        <BackHeader title="Video Not Found" />
        <View style={styles.errorContainer}>
          <Text style={styles.errorText}>Video not found or failed to load.</Text>
          <TouchableOpacity style={styles.retryButton} onPress={loadVideo}>
            <Text style={styles.retryButtonText}>Retry</Text>
          </TouchableOpacity>
        </View>
      </SafeAreaView>
    );
  }

  const statusInfo = getVideoStatus(video.Status);
  
  console.log('🎯 [VIDEO] Rendering video component with:', {
    videoID: video.ID,
    title: video.Title,
    status: video.Status,
    statusInfo: statusInfo,
    hlsPath: video.HLSPath,
    isCompleted: video.Status === 'COMPLETED',
    hasHLSPath: !!video.HLSPath
  });

  return (
    <SafeAreaView style={[styles.container, isFullscreen && styles.fullscreenContainer]}>
      {!isFullscreen && (
        <BackHeader 
          title={video.Title} 
          rightElement={
            <TouchableOpacity onPress={() => setShowQualitySelector(true)}>
              <Text style={styles.qualityText}>{quality.toUpperCase()}</Text>
            </TouchableOpacity>
          }
        />
      )}

      <View style={[styles.videoContainer, isFullscreen && styles.fullscreenVideoContainer]}>
        {video.Status === 'COMPLETED' && video.HLSPath ? (
          <>
            {console.log('🎥 [VIDEO] Rendering video player with stream URL:', `${API_BASE_URL}/stream/${id}`)}
            <VideoView
              style={styles.video}
              player={player}
              allowsFullscreen
              allowsPictureInPicture
              contentFit="contain"
            />
            
            {/* Debug info overlay - remove in production */}
            {__DEV__ && (
              <View style={styles.debugOverlay}>
                <Text style={styles.debugText}>
                  Status: {player.status} | Playing: {isPlaying ? 'Yes' : 'No'}
                </Text>
                <Text style={styles.debugText}>
                  Time: {formatTime(currentTime)} / {formatTime(duration)}
                </Text>
              </View>
            )}
            
            {/* Video Controls Overlay */}
            <TouchableOpacity 
              style={styles.videoOverlay} 
              onPress={handleVideoPress}
              activeOpacity={1}
            >
              {(showControls || buffering) && (
                <View style={styles.controlsContainer}>
                  {/* Top Controls */}
                  <View style={styles.topControls}>
                    {isFullscreen && (
                      <TouchableOpacity onPress={handleFullscreen}>
                        <Text style={styles.controlButton}>← Exit Fullscreen</Text>
                      </TouchableOpacity>
                    )}
                    <View style={styles.spacer} />
                    <Text style={styles.timeText}>
                      {formatTime(currentTime)} / {formatTime(duration)}
                    </Text>
                  </View>

                  {/* Center Play/Pause Button */}
                  <View style={styles.centerControls}>
                    {buffering ? (
                      <ActivityIndicator size="large" color="#fff" />
                    ) : (
                      <TouchableOpacity onPress={handlePlayPause} style={styles.playButton}>
                        <Text style={styles.playButtonText}>{isPlaying ? '⏸️' : '▶️'}</Text>
                      </TouchableOpacity>
                    )}
                  </View>

                  {/* Bottom Controls */}
                  <View style={styles.bottomControls}>
                    <Slider
                      style={styles.progressSlider}
                      minimumValue={0}
                      maximumValue={duration}
                      value={currentTime}
                      onValueChange={setCurrentTime}
                      onSlidingComplete={handleSeek}
                      minimumTrackTintColor="#007AFF"
                      maximumTrackTintColor="#333"
                    />
                    
                    <View style={styles.bottomControlsRow}>
                      <TouchableOpacity onPress={() => setShowQualitySelector(true)}>
                        <Text style={styles.controlButton}>Quality</Text>
                      </TouchableOpacity>
                      <View style={styles.spacer} />
                      <TouchableOpacity onPress={handleFullscreen}>
                        <Text style={styles.controlButton}>
                          {isFullscreen ? '⤓' : '⤢'}
                        </Text>
                      </TouchableOpacity>
                    </View>
                  </View>
                </View>
              )}
            </TouchableOpacity>
          </>
        ) : (
          <>
            {console.log('⚠️ [VIDEO] Rendering processing container - video not ready for playback:', {
              status: video.Status,
              hlsPath: video.HLSPath,
              error: video.Error,
              statusInfo: statusInfo
            })}
            <View style={styles.processingContainer}>
              <Text style={styles.processingTitle}>Video Processing</Text>
              <Text style={[styles.statusText, { color: statusInfo.color }]}>
                {statusInfo.text}
              </Text>
              {video.Status === 'PROCESSING' && (
                <ActivityIndicator size="large" color="#007AFF" style={styles.processingLoader} />
              )}
              <Text style={styles.processingDescription}>
                {video.Status === 'PENDING' 
                  ? 'Your video is queued for processing...'
                  : video.Status === 'PROCESSING'
                  ? 'Converting video for optimal streaming...'
                  : video.Error || 'Processing failed. Please try uploading again.'}
              </Text>
            </View>
          </>
        )}
      </View>

      {!isFullscreen && (
        <ScrollView style={styles.contentContainer} showsVerticalScrollIndicator={false}>
          {/* Video Information */}
          <View style={styles.infoSection}>
            <Text style={styles.videoTitle}>{video.Title}</Text>
            <View style={styles.metaRow}>
              <Text style={styles.viewCount}>{formatViewCount(video.ViewCount)}</Text>
              <Text style={styles.uploadDate}>
                {new Date(video.CreatedAt).toLocaleDateString()}
              </Text>
            </View>
          </View>

          {/* Video Description */}
          {video.Description && (
            <View style={styles.descriptionSection}>
              <Text style={styles.sectionTitle}>Description</Text>
              <Text style={styles.description}>{video.Description}</Text>
            </View>
          )}

          {/* Video Details */}
          <View style={styles.detailsSection}>
            <Text style={styles.sectionTitle}>Video Details</Text>
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Resolution:</Text>
              <Text style={styles.detailValue}>
                {video.Metadata.Width} × {video.Metadata.Height}
              </Text>
            </View>
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Duration:</Text>
              <Text style={styles.detailValue}>{formatTime(video.Metadata.Duration)}</Text>
            </View>
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Frame Rate:</Text>
              <Text style={styles.detailValue}>{video.Metadata.FrameRate} fps</Text>
            </View>
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Codec:</Text>
              <Text style={styles.detailValue}>
                {video.Metadata.Codec} / {video.Metadata.AudioCodec}
              </Text>
            </View>
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>File Size:</Text>
              <Text style={styles.detailValue}>
                {formatFileSize(video.Metadata.FileSize)}
              </Text>
            </View>
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Bitrate:</Text>
              <Text style={styles.detailValue}>{video.Metadata.Bitrate} kbps</Text>
            </View>
          </View>
        </ScrollView>
      )}

      {/* Quality Selector Modal */}
      <Modal
        visible={showQualitySelector}
        transparent
        animationType="slide"
        onRequestClose={() => setShowQualitySelector(false)}
      >
        <View style={styles.modalOverlay}>
          <View style={styles.qualityModal}>
            <Text style={styles.modalTitle}>Select Quality</Text>
            {['auto', '1080p', '720p', '480p', '360p'].map((q) => (
              <TouchableOpacity
                key={q}
                style={[styles.qualityOption, quality === q && styles.selectedQuality]}
                onPress={() => {
                  setQuality(q);
                  setShowQualitySelector(false);
                }}
              >
                <Text style={[styles.qualityOptionText, quality === q && styles.selectedQualityText]}>
                  {q.toUpperCase()}
                </Text>
              </TouchableOpacity>
            ))}
            <TouchableOpacity
              style={styles.cancelButton}
              onPress={() => setShowQualitySelector(false)}
            >
              <Text style={styles.cancelButtonText}>Cancel</Text>
            </TouchableOpacity>
          </View>
        </View>
      </Modal>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#000',
  },
  fullscreenContainer: {
    backgroundColor: '#000',
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f8f9fa',
  },
  loadingText: {
    marginTop: 16,
    fontSize: 16,
    color: '#666',
  },
  errorContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f8f9fa',
    padding: 20,
  },
  errorText: {
    fontSize: 16,
    color: '#666',
    textAlign: 'center',
    marginBottom: 20,
  },
  retryButton: {
    backgroundColor: '#007AFF',
    paddingHorizontal: 20,
    paddingVertical: 10,
    borderRadius: 8,
  },
  retryButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  qualityText: {
    fontSize: 14,
    color: '#007AFF',
    fontWeight: '500',
  },
  videoContainer: {
    width: screenWidth,
    height: videoHeight,
    backgroundColor: '#000',
    position: 'relative',
  },
  fullscreenVideoContainer: {
    flex: 1,
    width: '100%',
    height: '100%',
  },
  video: {
    width: '100%',
    height: '100%',
  },
  videoOverlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: 'space-between',
  },
  controlsContainer: {
    flex: 1,
    justifyContent: 'space-between',
    backgroundColor: 'rgba(0, 0, 0, 0.3)',
    padding: 16,
  },
  topControls: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  centerControls: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  bottomControls: {
    gap: 12,
  },
  bottomControlsRow: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  spacer: {
    flex: 1,
  },
  controlButton: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '500',
  },
  timeText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '500',
  },
  playButton: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: 'rgba(255, 255, 255, 0.2)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  playButtonText: {
    fontSize: 32,
  },
  progressSlider: {
    width: '100%',
    height: 40,
  },

  processingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 20,
    backgroundColor: '#f8f9fa',
  },
  processingTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  statusText: {
    fontSize: 16,
    fontWeight: '500',
    marginBottom: 16,
  },
  processingLoader: {
    marginVertical: 16,
  },
  processingDescription: {
    fontSize: 14,
    color: '#666',
    textAlign: 'center',
    lineHeight: 20,
  },
  contentContainer: {
    flex: 1,
    backgroundColor: '#f8f9fa',
  },
  infoSection: {
    padding: 16,
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#e1e5e9',
  },
  videoTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  metaRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  viewCount: {
    fontSize: 14,
    color: '#666',
  },
  uploadDate: {
    fontSize: 14,
    color: '#666',
  },
  descriptionSection: {
    padding: 16,
    backgroundColor: '#fff',
    marginTop: 8,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  description: {
    fontSize: 14,
    color: '#333',
    lineHeight: 20,
  },
  detailsSection: {
    padding: 16,
    backgroundColor: '#fff',
    marginTop: 8,
  },
  detailRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: 6,
  },
  detailLabel: {
    fontSize: 14,
    color: '#666',
  },
  detailValue: {
    fontSize: 14,
    color: '#1a1a1a',
    fontWeight: '500',
  },
  modalOverlay: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    justifyContent: 'flex-end',
  },
  qualityModal: {
    backgroundColor: '#fff',
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
    padding: 20,
  },
  modalTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 16,
    textAlign: 'center',
  },
  qualityOption: {
    paddingVertical: 12,
    paddingHorizontal: 16,
    borderRadius: 8,
    marginBottom: 8,
  },
  selectedQuality: {
    backgroundColor: '#007AFF',
  },
  qualityOptionText: {
    fontSize: 16,
    color: '#1a1a1a',
    textAlign: 'center',
  },
  selectedQualityText: {
    color: '#fff',
    fontWeight: '600',
  },
  cancelButton: {
    paddingVertical: 12,
    marginTop: 8,
  },
  cancelButtonText: {
    fontSize: 16,
    color: '#007AFF',
    textAlign: 'center',
  },
  
  // Debug styles
  debugOverlay: {
    position: 'absolute',
    top: 10,
    left: 10,
    right: 10,
    backgroundColor: 'rgba(0, 0, 0, 0.7)',
    padding: 8,
    borderRadius: 4,
  },
  debugText: {
    color: '#fff',
    fontSize: 12,
    fontFamily: 'monospace',
  },
});
