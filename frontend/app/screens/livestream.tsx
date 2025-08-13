import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Dimensions,
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Platform,
  StatusBar,
  FlatList,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router, useLocalSearchParams } from 'expo-router';
import AsyncStorage from '@react-native-async-storage/async-storage';
import BackHeader from '@/components/BackHeader';
import { useAuth } from '@/contexts/AuthContext';
import axios from 'axios';
import { API_BASE_URL } from '@/config/api';

// Types based on backend livestream struct
type LivestreamData = {
  ID: string;
  UserID: string;
  Title: string;
  Description: string;
  Status: 'OFFLINE' | 'LIVE' | 'ENDED';
  StreamKey: string;
  ViewerCount: number;
  PeakViewerCount: number;
  AverageViewerCount: number;
  StartedAt?: string;
  EndedAt?: string;
  CreatedAt: string;
  UpdatedAt: string;
};

type ChatMessage = {
  ID: string;
  StreamID: string;
  UserID: string;
  UserName: string;
  Message: string;
  CreatedAt: string;
};

type WebSocketMessage = {
  type: 'chat_message' | 'viewer_count_update' | 'stream_status_update' | 'webrtc_offer' | 'webrtc_answer' | 'webrtc_ice_candidate';
  payload: any;
};

const { width: screenWidth, height: screenHeight } = Dimensions.get('window');
const videoAspectRatio = 16 / 9;
const videoHeight = screenWidth / videoAspectRatio;

export default function LivestreamScreen() {
  const { logout } = useAuth();
  const { id } = useLocalSearchParams<{ id: string }>();
  
  const [stream, setStream] = useState<LivestreamData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isConnecting, setIsConnecting] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [newMessage, setNewMessage] = useState('');
  const [showChat, setShowChat] = useState(true);
  const [viewerCount, setViewerCount] = useState(0);
  const [isStreamLive, setIsStreamLive] = useState(false);
  
  const wsRef = useRef<WebSocket | null>(null);
  const chatScrollRef = useRef<FlatList>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [connectionAttempts, setConnectionAttempts] = useState(0);
  const MAX_RECONNECT_ATTEMPTS = 5;

  useEffect(() => {
    if (id) {
      loadStream();
    }
    
    return () => {
      cleanup();
    };
  }, [id]);

  const cleanup = () => {
    if (wsRef.current) {
      wsRef.current.close(1000, 'User disconnected');
      wsRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
  };

  const connectWebSocket = () => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    setIsConnecting(true);
    
    try {
      const wsUrl = `${API_BASE_URL.replace('http', 'ws')}/ws?stream_id=${id}`;
      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        console.log('WebSocket connected');
        setIsConnecting(false);
        setConnectionAttempts(0);
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          handleWebSocketMessage(message);
        } catch (error) {
          console.error('Error parsing WebSocket message:', error);
        }
      };

      wsRef.current.onclose = (event) => {
        console.log('WebSocket disconnected:', event.code, event.reason);
        setIsConnecting(false);
        
        // Attempt to reconnect if it wasn't a clean close
        if (event.code !== 1000) {
          setConnectionAttempts(prev => {
            if (prev < MAX_RECONNECT_ATTEMPTS) {
              reconnectTimeoutRef.current = setTimeout(() => {
                setConnectionAttempts(current => current + 1);
                connectWebSocket();
              }, Math.pow(2, prev) * 1000);
            }
            return prev + 1;
          });
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('WebSocket error:', error);
        setIsConnecting(false);
      };
    } catch (error) {
      console.error('Error creating WebSocket connection:', error);
      setIsConnecting(false);
    }
  };

  const loadStream = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      const response = await axios.get(`${API_BASE_URL}/api/livestream/status/${id}`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      setStream(response.data);
      setViewerCount(response.data.ViewerCount || 0);
      setIsStreamLive(response.data.Status === 'LIVE');
    } catch (error: any) {
      console.error('Error loading stream:', error);
      const errorMessage = error.response?.data?.error || 'Failed to load stream. Please try again.';
      Alert.alert('Error', errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (stream && stream.Status === 'LIVE') {
      connectWebSocket();
    }
    
    return () => {
      cleanup();
    };
  }, [stream]);

  const disconnectWebSocket = () => {
    if (wsRef.current) {
      wsRef.current.close(1000, 'User disconnected');
      wsRef.current = null;
    }
  };

  const handleWebSocketMessage = (message: WebSocketMessage) => {
    switch (message.type) {
      case 'chat_message':
        const chatMsg: ChatMessage = message.payload;
        setChatMessages(prev => [...prev, chatMsg]);
        // Auto-scroll to bottom
        setTimeout(() => {
          chatScrollRef.current?.scrollToEnd({ animated: true });
        }, 100);
        break;
        
      case 'viewer_count_update':
        setViewerCount(message.payload.count);
        break;
        
      case 'stream_status_update':
        const { status } = message.payload;
        setIsStreamLive(status === 'LIVE');
        if (status === 'ENDED' || status === 'OFFLINE') {
          Alert.alert('Stream Ended', 'This livestream has ended.');
        }
        break;
        
      case 'webrtc_answer':
        // Handle WebRTC answer for video streaming
        console.log('Received WebRTC answer:', message.payload);
        // In a real implementation, you'd handle this with a WebRTC peer connection
        break;
        
      default:
        console.log('Unknown message type:', message.type);
    }
  };

  const sendChatMessage = () => {
    if (!newMessage.trim() || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      return;
    }

    const message: WebSocketMessage = {
      type: 'chat_message',
      payload: {
        message: newMessage.trim(),
      },
    };

    wsRef.current.send(JSON.stringify(message));
    setNewMessage('');
  };

  const handleFullscreen = () => {
    setIsFullscreen(!isFullscreen);
    StatusBar.setHidden(!isFullscreen);
  };

  const formatViewerCount = (count: number): string => {
    if (count >= 1000000) {
      return `${(count / 1000000).toFixed(1)}M`;
    } else if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}K`;
    }
    return count.toString();
  };

  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const renderChatMessage = ({ item }: { item: ChatMessage }) => (
    <View style={styles.chatMessage}>
      <Text style={styles.chatUsername}>{item.UserName}</Text>
      <Text style={styles.chatText}>{item.Message}</Text>
      <Text style={styles.chatTime}>{formatTime(item.CreatedAt)}</Text>
    </View>
  );

  if (isLoading) {
    return (
      <SafeAreaView style={styles.container}>
        <BackHeader title="Loading..." />
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color="#007AFF" />
          <Text style={styles.loadingText}>Loading stream...</Text>
        </View>
      </SafeAreaView>
    );
  }

  if (!stream) {
    return (
      <SafeAreaView style={styles.container}>
        <BackHeader title="Stream Not Found" />
        <View style={styles.errorContainer}>
          <Text style={styles.errorText}>Stream not found or failed to load.</Text>
          <TouchableOpacity style={styles.retryButton} onPress={loadStream}>
            <Text style={styles.retryButtonText}>Retry</Text>
          </TouchableOpacity>
        </View>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={[styles.container, isFullscreen && styles.fullscreenContainer]}>
      {!isFullscreen && (
        <BackHeader 
          title={stream.Title}
          rightElement={
            <View style={styles.headerInfo}>
              <View style={[styles.liveIndicator, !isStreamLive && styles.offlineIndicator]}>
                <Text style={styles.liveText}>
                  {isStreamLive ? 'LIVE' : 'OFFLINE'}
                </Text>
              </View>
              <Text style={styles.viewerCountHeader}>
                {formatViewerCount(viewerCount)} viewers
              </Text>
            </View>
          }
        />
      )}

      <View style={[styles.videoContainer, isFullscreen && styles.fullscreenVideoContainer]}>
        {isStreamLive ? (
          <View style={styles.videoPlayer}>
            {/* WebRTC Video Stream Placeholder */}
            {/* In a real implementation, you'd use react-native-webrtc or similar */}
            <View style={styles.videoPlaceholder}>
              <Text style={styles.videoPlaceholderText}>📹</Text>
              <Text style={styles.videoStatusText}>Live Stream</Text>
              {isConnecting && (
                <ActivityIndicator size="large" color="#fff" style={styles.connectingLoader} />
              )}
            </View>
            
            {/* Stream Controls Overlay */}
            <View style={styles.streamOverlay}>
              <View style={styles.topStreamControls}>
                {isFullscreen && (
                  <TouchableOpacity onPress={handleFullscreen}>
                    <Text style={styles.controlButton}>← Exit Fullscreen</Text>
                  </TouchableOpacity>
                )}
                <View style={styles.spacer} />
                <View style={styles.liveInfo}>
                  <View style={styles.liveIndicator}>
                    <Text style={styles.liveText}>LIVE</Text>
                  </View>
                  <Text style={styles.viewerCount}>
                    {formatViewerCount(viewerCount)} watching
                  </Text>
                </View>
              </View>
              
              <View style={styles.bottomStreamControls}>
                <TouchableOpacity onPress={() => setShowChat(!showChat)}>
                  <Text style={styles.controlButton}>
                    {showChat ? 'Hide Chat' : 'Show Chat'}
                  </Text>
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
        ) : (
          <View style={styles.offlineContainer}>
            <Text style={styles.offlineTitle}>Stream Offline</Text>
            <Text style={styles.offlineDescription}>
              {stream.Status === 'ENDED' 
                ? 'This stream has ended.'
                : 'The streamer is currently offline.'}
            </Text>
            <TouchableOpacity style={styles.refreshButton} onPress={loadStream}>
              <Text style={styles.refreshButtonText}>Refresh</Text>
            </TouchableOpacity>
          </View>
        )}
      </View>

      {!isFullscreen && (
        <View style={styles.contentContainer}>
          {/* Stream Information */}
          <View style={styles.infoSection}>
            <Text style={styles.streamTitle}>{stream.Title}</Text>
            {stream.Description && (
              <Text style={styles.streamDescription}>{stream.Description}</Text>
            )}
            <View style={styles.streamMetaRow}>
              <Text style={styles.streamMeta}>
                Started: {stream.StartedAt ? new Date(stream.StartedAt).toLocaleString() : 'N/A'}
              </Text>
              <Text style={styles.streamMeta}>
                Peak: {formatViewerCount(stream.PeakViewerCount)} viewers
              </Text>
            </View>
          </View>

          {/* Chat Section */}
          {showChat && isStreamLive && (
            <View style={styles.chatSection}>
              <Text style={styles.chatTitle}>Live Chat</Text>
              
              <FlatList
                ref={chatScrollRef}
                data={chatMessages}
                renderItem={renderChatMessage}
                keyExtractor={(item) => item.ID}
                style={styles.chatList}
                showsVerticalScrollIndicator={false}
                onContentSizeChange={() => chatScrollRef.current?.scrollToEnd({ animated: true })}
              />
              
              <KeyboardAvoidingView
                behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
                style={styles.chatInputContainer}
              >
                <TextInput
                  style={styles.chatInput}
                  value={newMessage}
                  onChangeText={setNewMessage}
                  placeholder="Send a message..."
                  placeholderTextColor="#999"
                  multiline
                  maxLength={500}
                  editable={wsRef.current?.readyState === WebSocket.OPEN}
                />
                <TouchableOpacity
                  style={[
                    styles.sendButton,
                    (!newMessage.trim() || wsRef.current?.readyState !== WebSocket.OPEN) && styles.sendButtonDisabled
                  ]}
                  onPress={sendChatMessage}
                  disabled={!newMessage.trim() || wsRef.current?.readyState !== WebSocket.OPEN}
                >
                  <Text style={styles.sendButtonText}>Send</Text>
                </TouchableOpacity>
              </KeyboardAvoidingView>
            </View>
          )}
          
          {/* Connection Status */}
          {isStreamLive && (
            <View style={styles.connectionStatus}>
              <View style={[
                styles.connectionIndicator,
                wsRef.current?.readyState === WebSocket.OPEN 
                  ? styles.connectedIndicator 
                  : styles.disconnectedIndicator
              ]} />
              <Text style={styles.connectionText}>
                {wsRef.current?.readyState === WebSocket.OPEN 
                  ? 'Connected' 
                  : isConnecting 
                  ? 'Connecting...' 
                  : 'Disconnected'}
              </Text>
            </View>
          )}
        </View>
      )}
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
  headerInfo: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  liveIndicator: {
    backgroundColor: '#FF3B30',
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 4,
  },
  offlineIndicator: {
    backgroundColor: '#8E8E93',
  },
  liveText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: '600',
  },
  viewerCountHeader: {
    fontSize: 12,
    color: '#666',
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
  videoPlayer: {
    width: '100%',
    height: '100%',
    position: 'relative',
  },
  videoPlaceholder: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#1a1a1a',
  },
  videoPlaceholderText: {
    fontSize: 64,
    marginBottom: 16,
  },
  videoStatusText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
  },
  connectingLoader: {
    marginTop: 20,
  },
  streamOverlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: 'space-between',
    backgroundColor: 'rgba(0, 0, 0, 0.2)',
    padding: 16,
  },
  topStreamControls: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  bottomStreamControls: {
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
  liveInfo: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  viewerCount: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '500',
  },
  offlineContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f8f9fa',
    padding: 20,
  },
  offlineTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  offlineDescription: {
    fontSize: 14,
    color: '#666',
    textAlign: 'center',
    marginBottom: 20,
  },
  refreshButton: {
    backgroundColor: '#007AFF',
    paddingHorizontal: 20,
    paddingVertical: 10,
    borderRadius: 8,
  },
  refreshButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
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
  streamTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  streamDescription: {
    fontSize: 14,
    color: '#333',
    lineHeight: 20,
    marginBottom: 12,
  },
  streamMetaRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  streamMeta: {
    fontSize: 12,
    color: '#666',
  },
  chatSection: {
    flex: 1,
    backgroundColor: '#fff',
    marginTop: 8,
  },
  chatTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1a1a1a',
    padding: 16,
    paddingBottom: 8,
  },
  chatList: {
    flex: 1,
    paddingHorizontal: 16,
    maxHeight: 200,
  },
  chatMessage: {
    marginBottom: 12,
    paddingBottom: 8,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  chatUsername: {
    fontSize: 14,
    fontWeight: '600',
    color: '#007AFF',
    marginBottom: 2,
  },
  chatText: {
    fontSize: 14,
    color: '#1a1a1a',
    lineHeight: 18,
    marginBottom: 4,
  },
  chatTime: {
    fontSize: 11,
    color: '#999',
  },
  chatInputContainer: {
    flexDirection: 'row',
    padding: 16,
    paddingTop: 8,
    borderTopWidth: 1,
    borderTopColor: '#e1e5e9',
    alignItems: 'flex-end',
  },
  chatInput: {
    flex: 1,
    borderWidth: 1,
    borderColor: '#e1e5e9',
    borderRadius: 20,
    paddingHorizontal: 16,
    paddingVertical: 8,
    marginRight: 8,
    maxHeight: 100,
    fontSize: 14,
    backgroundColor: '#f8f9fa',
  },
  sendButton: {
    backgroundColor: '#007AFF',
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 20,
  },
  sendButtonDisabled: {
    backgroundColor: '#ccc',
  },
  sendButtonText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
  },
  connectionStatus: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    backgroundColor: '#fff',
    marginTop: 8,
  },
  connectionIndicator: {
    width: 8,
    height: 8,
    borderRadius: 4,
    marginRight: 8,
  },
  connectedIndicator: {
    backgroundColor: '#34C759',
  },
  disconnectedIndicator: {
    backgroundColor: '#FF3B30',
  },
  connectionText: {
    fontSize: 12,
    color: '#666',
  },
});
