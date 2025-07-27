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
import { Video, ResizeMode } from 'expo-av';
import Slider from '@react-native-community/slider';
import BackHeader from '@/components/BackHeader';
import { useAuth } from '@/contexts/AuthContext';

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
  const videoRef = useRef<Video>(null);
  
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

  useEffect(() => {
    if (id) {
      loadVideo();
    }
  }, [id]);

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
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      const response = await fetch(`http://localhost:8080/api/video/${id}`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (response.ok) {
        const videoData = await response.json();
        setVideo(videoData);
        setDuration(videoData.Metadata.Duration);
      } else {
        throw new Error('Failed to load video');
      }
    } catch (error) {
      console.error('Error loading video:', error);
      Alert.alert('Error', 'Failed to load video. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  const handlePlayPause = async () => {
    if (videoRef.current) {
      if (isPlaying) {
        await videoRef.current.pauseAsync();
      } else {
        await videoRef.current.playAsync();
      }
      setIsPlaying(!isPlaying);
    }
  };

  const handleSeek = async (time: number) => {
    if (videoRef.current && video) {
      await videoRef.current.setPositionAsync(time * 1000);
      setCurrentTime(time);
      
      // Update backend with seek time for analytics
      try {
        const token = await AsyncStorage.getItem('userToken');
        if (token) {
          await fetch(`http://localhost:8080/video/${id}/timestamp?current=${time}`, {
            headers: { 'Authorization': `Bearer ${token}` },
          });
        }
      } catch (error) {
        console.log('Failed to update timestamp:', error);
      }
    }
  };

  const handleVideoPress = () => {
    setShowControls(!showControls);
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
            <Video
              ref={videoRef}
              style={styles.video}
              source={{ uri: `http://localhost:8080/stream/${id}` }}
              shouldPlay={false}
              isLooping={false}
              resizeMode={ResizeMode.CONTAIN}
              onPlaybackStatusUpdate={(status: any) => {
                if (status.isLoaded) {
                  setCurrentTime(status.positionMillis / 1000);
                  setIsPlaying(status.isPlaying);
                  setBuffering(status.isBuffering);
                  if (status.durationMillis) {
                    setDuration(status.durationMillis / 1000);
                  }
                }
              }}
            />
            
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
});
