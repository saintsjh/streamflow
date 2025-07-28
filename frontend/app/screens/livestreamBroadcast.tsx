import React, { useState, useEffect, useRef } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Alert,
  Dimensions,
  StatusBar,
  ActivityIndicator,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router, useLocalSearchParams } from 'expo-router';
import { Camera, CameraType, CameraView } from 'expo-camera';
import * as MediaLibrary from 'expo-media-library';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useAuth } from '@/contexts/AuthContext';
import Constants from 'expo-constants';
import {
  StreamMessageType,
  createStreamMessage
} from '@/utils/streamUtils';

const { width: screenWidth, height: screenHeight } = Dimensions.get('window');

type StreamStatus = 'PREPARING' | 'LIVE' | 'STOPPED' | 'ERROR';

type LivestreamData = {
  ID: string;
  Title: string;
  StreamKey: string;
  Status: string;
};

export default function LivestreamBroadcastScreen() {
  const { logout } = useAuth();
  const { id } = useLocalSearchParams<{ id: string }>();
  
  const [stream, setStream] = useState<LivestreamData | null>(null);
  const [cameraPermission, setCameraPermission] = useState<boolean>(false);
  const [microphonePermission, setMicrophonePermission] = useState<boolean>(false);
  const [mediaLibraryPermission, setMediaLibraryPermission] = useState<boolean>(false);
  
  const [facing, setFacing] = useState<CameraType>('back');
  const [isRecording, setIsRecording] = useState(false);
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('PREPARING');
  const [isLoading, setIsLoading] = useState(true);
  const [viewerCount, setViewerCount] = useState(0);
  const [streamDuration, setStreamDuration] = useState(0);
  
  const cameraRef = useRef<CameraView>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const streamStartTime = useRef<Date | null>(null);
  const durationInterval = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    requestPermissions();
    if (id) {
      loadStreamDetails();
    }
    
    return () => {
      cleanup();
    };
  }, [id]);

  useEffect(() => {
    if (streamStatus === 'LIVE') {
      startDurationTimer();
    } else {
      stopDurationTimer();
    }
  }, [streamStatus]);

  const cleanup = () => {
    if (wsRef.current) {
      wsRef.current.close();
    }
    stopDurationTimer();
    if (isRecording) {
      stopRecording();
    }
  };

  const requestPermissions = async () => {
    try {
      const [cameraStatus, microphoneStatus, mediaLibraryStatus] = await Promise.all([
        Camera.requestCameraPermissionsAsync(),
        Camera.requestMicrophonePermissionsAsync(),
        MediaLibrary.requestPermissionsAsync(),
      ]);

      setCameraPermission(cameraStatus.status === 'granted');
      setMicrophonePermission(microphoneStatus.status === 'granted');
      setMediaLibraryPermission(mediaLibraryStatus.status === 'granted');

      if (cameraStatus.status !== 'granted' || microphoneStatus.status !== 'granted') {
        Alert.alert(
          'Permissions Required',
          'Camera and microphone permissions are required to start a livestream.',
          [
            { text: 'Cancel', onPress: () => router.back() },
            { text: 'Settings', onPress: () => {/* Open settings */} },
          ]
        );
      }
    } catch (error) {
      console.error('Error requesting permissions:', error);
      Alert.alert('Error', 'Failed to request permissions.');
    }
  };

  const loadStreamDetails = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      const response = await fetch(`${Constants.expoConfig?.extra?.apiBaseUrl || process.env.EXPO_PUBLIC_API_BASE_URL}/api/livestream/status/${id}`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (response.ok) {
        const streamData = await response.json();
        setStream(streamData);
        if (streamData.Status === 'LIVE') {
          setStreamStatus('LIVE');
          connectWebSocket();
        }
      } else {
        throw new Error('Failed to load stream details');
      }
    } catch (error) {
      console.error('Error loading stream:', error);
      setStreamStatus('ERROR');
    } finally {
      setIsLoading(false);
    }
  };

  const connectWebSocket = () => {
    try {
      const apiBaseUrl = Constants.expoConfig?.extra?.apiBaseUrl || process.env.EXPO_PUBLIC_API_BASE_URL;
      const wsUrl = `${apiBaseUrl?.replace('http', 'ws')}/ws?stream_id=${id}`;
      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        console.log('WebSocket connected for broadcasting');
        // Send stream start message
        const message = createStreamMessage(StreamMessageType.STREAM_START, {
          streamId: id,
          timestamp: Date.now(),
        });
        wsRef.current?.send(JSON.stringify(message));
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          if (message.type === 'viewer_count_update') {
            setViewerCount(message.payload.count);
          }
        } catch (error) {
          console.error('Error parsing WebSocket message:', error);
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      wsRef.current.onclose = () => {
        console.log('WebSocket disconnected');
      };
    } catch (error) {
      console.error('Error connecting WebSocket:', error);
    }
  };

  const startDurationTimer = () => {
    streamStartTime.current = new Date();
    durationInterval.current = setInterval(() => {
      if (streamStartTime.current) {
        const elapsed = Math.floor((Date.now() - streamStartTime.current.getTime()) / 1000);
        setStreamDuration(elapsed);
      }
    }, 1000);
  };

  const stopDurationTimer = () => {
    if (durationInterval.current) {
      clearInterval(durationInterval.current);
      durationInterval.current = null;
    }
  };

  const toggleCameraFacing = () => {
    setFacing(current => (current === 'back' ? 'front' : 'back'));
  };

  const startRecording = async () => {
    if (!cameraRef.current || !cameraPermission || !microphonePermission) {
      Alert.alert('Error', 'Camera permissions are required to record.');
      return;
    }

    try {
      setIsRecording(true);
      
      const recordingOptions = {
        quality: '720p' as const,
        maxDuration: 60, // 60 seconds max for mobile streaming
      };

      const recording = await cameraRef.current.recordAsync(recordingOptions);
      
      // In a real implementation, you would stream this data via WebRTC
      console.log('Recording started, video will be saved to:', recording.uri);
      
      // Simulate streaming - in production you'd send video chunks via WebSocket/WebRTC
      if (streamStatus === 'PREPARING') {
        setStreamStatus('LIVE');
        connectWebSocket();
      }
      
    } catch (error) {
      console.error('Error starting recording:', error);
      setIsRecording(false);
      Alert.alert('Error', 'Failed to start recording.');
    }
  };

  const stopRecording = async () => {
    if (!cameraRef.current || !isRecording) return;

    try {
      cameraRef.current.stopRecording();
      setIsRecording(false);
      
      if (streamStatus === 'LIVE') {
        setStreamStatus('STOPPED');
      }
    } catch (error) {
      console.error('Error stopping recording:', error);
    }
  };

  const endStream = async () => {
    Alert.alert(
      'End Stream',
      'Are you sure you want to end your livestream?',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'End Stream',
          style: 'destructive',
          onPress: async () => {
            try {
              if (isRecording) {
                await stopRecording();
              }
              
              // Send stream end message via WebSocket
              if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
                const message = createStreamMessage(StreamMessageType.STREAM_END, {
                  streamId: id,
                  timestamp: Date.now(),
                });
                wsRef.current.send(JSON.stringify(message));
              }
              
              const token = await AsyncStorage.getItem('userToken');
              if (token) {
                await fetch(`${Constants.expoConfig?.extra?.apiBaseUrl || process.env.EXPO_PUBLIC_API_BASE_URL}/api/livestream/stop/${id}`, {
                  method: 'POST',
                  headers: {
                    'Authorization': `Bearer ${token}`,
                  },
                });
              }
              
              setStreamStatus('STOPPED');
              router.back();
            } catch (error) {
              console.error('Error ending stream:', error);
              Alert.alert('Error', 'Failed to end stream properly.');
            }
          },
        },
      ]
    );
  };

  const formatDuration = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    
    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${secs.toString().padStart(2, '0')}`;
  };

  if (isLoading) {
    return (
      <SafeAreaView style={styles.container}>
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color="#007AFF" />
          <Text style={styles.loadingText}>Setting up your stream...</Text>
        </View>
      </SafeAreaView>
    );
  }

  if (!cameraPermission || !microphonePermission) {
    return (
      <SafeAreaView style={styles.container}>
        <View style={styles.permissionContainer}>
          <Text style={styles.permissionTitle}>Permissions Required</Text>
          <Text style={styles.permissionText}>
            Camera and microphone access are required to broadcast your livestream.
          </Text>
          <TouchableOpacity style={styles.permissionButton} onPress={requestPermissions}>
            <Text style={styles.permissionButtonText}>Grant Permissions</Text>
          </TouchableOpacity>
        </View>
      </SafeAreaView>
    );
  }

  return (
    <View style={styles.container}>
      <StatusBar barStyle="light-content" backgroundColor="#000" />
      
      {/* Camera View */}
      <CameraView
        ref={cameraRef}
        style={styles.camera}
        facing={facing}
        mode="video"
      >
        {/* Top Overlay */}
        <View style={styles.topOverlay}>
          <View style={styles.streamInfo}>
            <View style={[styles.liveIndicator, streamStatus === 'LIVE' && styles.liveIndicatorActive]}>
              <Text style={styles.liveText}>
                {streamStatus === 'LIVE' ? 'LIVE' : streamStatus}
              </Text>
            </View>
            {streamStatus === 'LIVE' && (
              <Text style={styles.durationText}>{formatDuration(streamDuration)}</Text>
            )}
          </View>
          
          <TouchableOpacity style={styles.closeButton} onPress={() => router.back()}>
            <Text style={styles.closeButtonText}>✕</Text>
          </TouchableOpacity>
        </View>

        {/* Bottom Overlay */}
        <View style={styles.bottomOverlay}>
          <View style={styles.streamStats}>
            <Text style={styles.streamTitle}>{stream?.Title || 'My Stream'}</Text>
            <Text style={styles.viewerCount}>👥 {viewerCount} viewers</Text>
          </View>
          
          <View style={styles.controls}>
            {/* Camera Flip Button */}
            <TouchableOpacity
              style={styles.controlButton}
              onPress={toggleCameraFacing}
              disabled={isRecording}
            >
              <Text style={styles.controlButtonText}>🔄</Text>
            </TouchableOpacity>

            {/* Record/Stop Button */}
            <TouchableOpacity
              style={[
                styles.recordButton,
                isRecording && styles.recordButtonActive,
              ]}
              onPress={isRecording ? stopRecording : startRecording}
            >
              <View style={[
                styles.recordButtonInner,
                isRecording && styles.recordButtonInnerActive,
              ]} />
            </TouchableOpacity>

            {/* End Stream Button */}
            <TouchableOpacity
              style={styles.controlButton}
              onPress={endStream}
            >
              <Text style={styles.controlButtonText}>⏹️</Text>
            </TouchableOpacity>
          </View>
        </View>
      </CameraView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#000',
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#000',
  },
  loadingText: {
    color: '#fff',
    fontSize: 16,
    marginTop: 16,
  },
  permissionContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#000',
    padding: 20,
  },
  permissionTitle: {
    color: '#fff',
    fontSize: 24,
    fontWeight: 'bold',
    marginBottom: 16,
    textAlign: 'center',
  },
  permissionText: {
    color: '#ccc',
    fontSize: 16,
    textAlign: 'center',
    marginBottom: 32,
    lineHeight: 24,
  },
  permissionButton: {
    backgroundColor: '#007AFF',
    paddingHorizontal: 32,
    paddingVertical: 16,
    borderRadius: 12,
  },
  permissionButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  camera: {
    flex: 1,
  },
  topOverlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    padding: 20,
    paddingTop: 60,
    background: 'linear-gradient(180deg, rgba(0,0,0,0.6) 0%, transparent 100%)',
  },
  streamInfo: {
    alignItems: 'flex-start',
  },
  liveIndicator: {
    backgroundColor: 'rgba(255, 255, 255, 0.3)',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 16,
    marginBottom: 8,
  },
  liveIndicatorActive: {
    backgroundColor: '#FF3B30',
  },
  liveText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: 'bold',
  },
  durationText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
  },
  closeButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  closeButtonText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: 'bold',
  },
  bottomOverlay: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    padding: 20,
    paddingBottom: 40,
    background: 'linear-gradient(0deg, rgba(0,0,0,0.6) 0%, transparent 100%)',
  },
  streamStats: {
    marginBottom: 20,
    alignItems: 'center',
  },
  streamTitle: {
    color: '#fff',
    fontSize: 18,
    fontWeight: 'bold',
    marginBottom: 4,
    textAlign: 'center',
  },
  viewerCount: {
    color: '#ccc',
    fontSize: 14,
  },
  controls: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 40,
  },
  controlButton: {
    width: 60,
    height: 60,
    borderRadius: 30,
    backgroundColor: 'rgba(255, 255, 255, 0.2)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  controlButtonText: {
    fontSize: 24,
  },
  recordButton: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: 'rgba(255, 255, 255, 0.3)',
    justifyContent: 'center',
    alignItems: 'center',
    borderWidth: 4,
    borderColor: '#fff',
  },
  recordButtonActive: {
    borderColor: '#FF3B30',
  },
  recordButtonInner: {
    width: 30,
    height: 30,
    borderRadius: 15,
    backgroundColor: '#FF3B30',
  },
  recordButtonInnerActive: {
    borderRadius: 4,
    width: 20,
    height: 20,
  },
});