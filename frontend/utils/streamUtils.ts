/**
 * Utility functions for handling livestream video and audio data
 */

export interface StreamConfig {
  quality: 'LOW' | 'MEDIUM' | 'HIGH' | 'ULTRA';
  enableAudio: boolean;
  enableVideo: boolean;
  maxDuration?: number;
}

export interface VideoChunk {
  data: Uint8Array;
  timestamp: number;
  isKeyFrame: boolean;
}

export interface AudioChunk {
  data: Uint8Array;
  timestamp: number;
  sampleRate: number;
}

/**
 * Get video recording options based on quality setting
 */
export const getRecordingOptions = (quality: StreamConfig['quality']) => {
  const qualityMap = {
    LOW: {
      quality: '720p' as const,
      videoBitrate: 1000000, // 1 Mbps
      audioBitrate: 64000,   // 64 kbps
    },
    MEDIUM: {
      quality: '1080p' as const,
      videoBitrate: 2500000, // 2.5 Mbps
      audioBitrate: 128000,  // 128 kbps
    },
    HIGH: {
      quality: '1080p' as const,
      videoBitrate: 5000000, // 5 Mbps
      audioBitrate: 192000,  // 192 kbps
    },
    ULTRA: {
      quality: '2160p' as const,
      videoBitrate: 15000000, // 15 Mbps
      audioBitrate: 256000,   // 256 kbps
    },
  };

  return qualityMap[quality];
};

/**
 * Create WebRTC configuration for peer connection
 */
export const getWebRTCConfig = () => {
  return {
    iceServers: [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
    ],
    iceCandidatePoolSize: 10,
  };
};

/**
 * Format streaming duration in HH:MM:SS or MM:SS format
 */
export const formatStreamDuration = (seconds: number): string => {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  
  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  }
  return `${minutes}:${secs.toString().padStart(2, '0')}`;
};

/**
 * Validate stream configuration
 */
export const validateStreamConfig = (config: StreamConfig): string[] => {
  const errors: string[] = [];
  
  if (!config.enableVideo && !config.enableAudio) {
    errors.push('At least video or audio must be enabled');
  }
  
  if (config.maxDuration && config.maxDuration < 10) {
    errors.push('Stream duration must be at least 10 seconds');
  }
  
  return errors;
};

/**
 * Calculate estimated bandwidth usage
 */
export const estimateBandwidth = (quality: StreamConfig['quality']): string => {
  const options = getRecordingOptions(quality);
  const totalBitrate = options.videoBitrate + options.audioBitrate;
  const mbps = (totalBitrate / 1000000).toFixed(1);
  return `${mbps} Mbps`;
};

/**
 * WebSocket message types for streaming
 */
export enum StreamMessageType {
  VIDEO_CHUNK = 'video_chunk',
  AUDIO_CHUNK = 'audio_chunk',
  STREAM_START = 'stream_start',
  STREAM_END = 'stream_end',
  VIEWER_COUNT = 'viewer_count_update',
  CHAT_MESSAGE = 'chat_message',
  WEBRTC_OFFER = 'webrtc_offer',
  WEBRTC_ANSWER = 'webrtc_answer',
  WEBRTC_ICE_CANDIDATE = 'webrtc_ice_candidate',
}

export interface StreamMessage {
  type: StreamMessageType;
  payload: any;
  timestamp: number;
}

/**
 * Create a stream message
 */
export const createStreamMessage = (type: StreamMessageType, payload: any): StreamMessage => {
  return {
    type,
    payload,
    timestamp: Date.now(),
  };
};

/**
 * Send video chunk via WebSocket
 */
export const sendVideoChunk = (
  ws: WebSocket,
  chunk: VideoChunk,
  streamId: string
) => {
  if (ws.readyState === WebSocket.OPEN) {
    const message = createStreamMessage(StreamMessageType.VIDEO_CHUNK, {
      streamId,
      data: Array.from(chunk.data), // Convert Uint8Array to regular array for JSON
      timestamp: chunk.timestamp,
      isKeyFrame: chunk.isKeyFrame,
    });
    ws.send(JSON.stringify(message));
  }
};

/**
 * Send audio chunk via WebSocket
 */
export const sendAudioChunk = (
  ws: WebSocket,
  chunk: AudioChunk,
  streamId: string
) => {
  if (ws.readyState === WebSocket.OPEN) {
    const message = createStreamMessage(StreamMessageType.AUDIO_CHUNK, {
      streamId,
      data: Array.from(chunk.data), // Convert Uint8Array to regular array for JSON
      timestamp: chunk.timestamp,
      sampleRate: chunk.sampleRate,
    });
    ws.send(JSON.stringify(message));
  }
};

/**
 * Create WebRTC offer for streaming
 */
export const createWebRTCOffer = async (
  peerConnection: RTCPeerConnection
): Promise<RTCSessionDescriptionInit> => {
  const offer = await peerConnection.createOffer({
    offerToReceiveAudio: false,
    offerToReceiveVideo: false,
  });
  await peerConnection.setLocalDescription(offer);
  return offer;
};

/**
 * Handle WebRTC answer from server
 */
export const handleWebRTCAnswer = async (
  peerConnection: RTCPeerConnection,
  answer: RTCSessionDescriptionInit
) => {
  await peerConnection.setRemoteDescription(new RTCSessionDescription(answer));
};

/**
 * Add ICE candidate to peer connection
 */
export const addICECandidate = async (
  peerConnection: RTCPeerConnection,
  candidate: RTCIceCandidateInit
) => {
  await peerConnection.addIceCandidate(new RTCIceCandidate(candidate));
};