import React, { useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router } from 'expo-router';
import * as DocumentPicker from 'expo-document-picker';
import AsyncStorage from '@react-native-async-storage/async-storage';
import BackHeader from '@/components/BackHeader';
import { useAuth } from '@/contexts/AuthContext';
import Constants from 'expo-constants';

// Types based on backend validation
type VideoUploadData = {
  title: string;
  description: string;
  videoFile: DocumentPicker.DocumentPickerAsset | null;
};

const MAX_FILE_SIZE = 500 * 1024 * 1024; // 500MB
const ALLOWED_VIDEO_TYPES = ['video/mp4', 'video/avi', 'video/mov', 'video/quicktime', 'video/x-msvideo', 'video/x-matroska', 'video/webm'];
const ALLOWED_EXTENSIONS = ['.mp4', '.avi', '.mov', '.mkv', '.webm'];

export default function UploadVideoScreen() {
  const { logout } = useAuth();
  const [isLoading, setIsLoading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [errors, setErrors] = useState<Record<string, string>>({});
  
  const [uploadData, setUploadData] = useState<VideoUploadData>({
    title: '',
    description: '',
    videoFile: null,
  });

  // Validation rules
  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!uploadData.title.trim()) {
      newErrors.title = 'Title is required';
    } else if (uploadData.title.length < 3) {
      newErrors.title = 'Title must be at least 3 characters';
    } else if (uploadData.title.length > 100) {
      newErrors.title = 'Title must be less than 100 characters';
    }

    if (uploadData.description.length > 500) {
      newErrors.description = 'Description must be less than 500 characters';
    }

    if (!uploadData.videoFile) {
      newErrors.videoFile = 'Please select a video file';
    } else {
      // File size validation
      if (uploadData.videoFile.size && uploadData.videoFile.size > MAX_FILE_SIZE) {
        newErrors.videoFile = `File size must be less than ${MAX_FILE_SIZE / (1024 * 1024)}MB`;
      }

      // File type validation
      const fileExtension = uploadData.videoFile.name?.toLowerCase().split('.').pop();
      if (fileExtension && !ALLOWED_EXTENSIONS.includes(`.${fileExtension}`)) {
        newErrors.videoFile = `File type not supported. Allowed: ${ALLOWED_EXTENSIONS.join(', ')}`;
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handlePickVideo = async () => {
    try {
      const result = await DocumentPicker.getDocumentAsync({
        type: 'video/*',
        copyToCacheDirectory: true,
      });

      if (!result.canceled && result.assets && result.assets.length > 0) {
        const asset = result.assets[0];
        
        // Additional validation
        if (asset.size && asset.size > MAX_FILE_SIZE) {
          Alert.alert('File Too Large', `Please select a video file smaller than ${MAX_FILE_SIZE / (1024 * 1024)}MB`);
          return;
        }

        setUploadData({ ...uploadData, videoFile: asset });
        
        // Clear file error if it exists
        if (errors.videoFile) {
          setErrors({ ...errors, videoFile: '' });
        }
      }
    } catch (error) {
      console.error('Error picking video:', error);
      Alert.alert('Error', 'Failed to select video file. Please try again.');
    }
  };

  const handleUploadVideo = async () => {
    if (!validateForm()) {
      Alert.alert('Validation Error', 'Please fix the errors before uploading your video.');
      return;
    }

    setIsLoading(true);
    setUploadProgress(0);
    
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      const formData = new FormData();
      formData.append('title', uploadData.title.trim());
      formData.append('description', uploadData.description.trim());
      
      // Append the video file
      formData.append('video', {
        uri: uploadData.videoFile!.uri,
        type: uploadData.videoFile!.mimeType || 'video/mp4',
        name: uploadData.videoFile!.name || 'video.mp4',
      } as any);

      const response = await fetch(`${Constants.expoConfig?.extra?.apiBaseUrl || process.env.EXPO_PUBLIC_API_BASE_URL}/api/video/upload`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'multipart/form-data',
        },
        body: formData,
      });

      const result = await response.json();

      if (response.ok) {
        Alert.alert(
          'Upload Successful! 🎉',
          `Your video "${uploadData.title}" has been uploaded successfully and will be processed shortly.`,
          [
            {
              text: 'View Videos',
              onPress: () => {
                router.replace('/(tabs)');
              },
            },
          ]
        );
      } else {
        throw new Error(result.error || 'Upload failed');
      }
    } catch (error) {
      console.error('Error uploading video:', error);
      Alert.alert(
        'Upload Error',
        'Failed to upload your video. Please check your connection and try again.',
        [{ text: 'OK' }]
      );
    } finally {
      setIsLoading(false);
      setUploadProgress(0);
    }
  };

  const handleSaveDraft = async () => {
    try {
      // Save form data as draft (could be implemented with AsyncStorage)
      console.log('Saving draft:', uploadData);
      Alert.alert('Draft Saved', 'Your video upload draft has been saved.');
    } catch (error) {
      console.error('Error saving draft:', error);
    }
  };

  const formatFileSize = (bytes: number): string => {
    const MB = bytes / (1024 * 1024);
    return `${MB.toFixed(1)} MB`;
  };

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <BackHeader
        title="Upload Video"
        subtitle="Share your content with the world"
        rightElement={
          <TouchableOpacity style={styles.draftButton} onPress={handleSaveDraft}>
            <Text style={styles.draftButtonText}>Save Draft</Text>
          </TouchableOpacity>
        }
      />

      <KeyboardAvoidingView
        style={styles.keyboardView}
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
      >
        <ScrollView
          style={styles.scrollView}
          showsVerticalScrollIndicator={false}
          keyboardShouldPersistTaps="handled"
        >
          {/* Video File Selection */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Video File</Text>
            
            <TouchableOpacity
              style={[styles.filePickerButton, errors.videoFile && styles.filePickerError]}
              onPress={handlePickVideo}
              disabled={isLoading}
            >
              {uploadData.videoFile ? (
                <View style={styles.fileInfo}>
                  <Text style={styles.fileName}>{uploadData.videoFile.name}</Text>
                  <Text style={styles.fileSize}>
                    {uploadData.videoFile.size ? formatFileSize(uploadData.videoFile.size) : 'Unknown size'}
                  </Text>
                </View>
              ) : (
                <View style={styles.filePickerContent}>
                  <Text style={styles.filePickerIcon}>📹</Text>
                  <Text style={styles.filePickerText}>Tap to select video</Text>
                  <Text style={styles.filePickerSubtext}>
                    Max 500MB • MP4, AVI, MOV, MKV, WEBM
                  </Text>
                </View>
              )}
            </TouchableOpacity>
            
            {errors.videoFile && <Text style={styles.errorText}>{errors.videoFile}</Text>}
          </View>

          {/* Basic Information */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Video Information</Text>
            
            <View style={styles.inputContainer}>
              <Text style={styles.inputLabel}>Title *</Text>
              <TextInput
                style={[styles.textInput, errors.title && styles.textInputError]}
                value={uploadData.title}
                onChangeText={(text) => {
                  setUploadData({ ...uploadData, title: text });
                  if (errors.title) {
                    setErrors({ ...errors, title: '' });
                  }
                }}
                placeholder="Enter your video title"
                placeholderTextColor="#999"
                maxLength={100}
                editable={!isLoading}
              />
              {errors.title && <Text style={styles.errorText}>{errors.title}</Text>}
              <Text style={styles.characterCount}>{uploadData.title.length}/100</Text>
            </View>

            <View style={styles.inputContainer}>
              <Text style={styles.inputLabel}>Description</Text>
              <TextInput
                style={[styles.textInput, styles.textInputMultiline, errors.description && styles.textInputError]}
                value={uploadData.description}
                onChangeText={(text) => {
                  setUploadData({ ...uploadData, description: text });
                  if (errors.description) {
                    setErrors({ ...errors, description: '' });
                  }
                }}
                placeholder="Describe your video (optional)"
                placeholderTextColor="#999"
                multiline
                numberOfLines={4}
                maxLength={500}
                textAlignVertical="top"
                editable={!isLoading}
              />
              {errors.description && <Text style={styles.errorText}>{errors.description}</Text>}
              <Text style={styles.characterCount}>{uploadData.description.length}/500</Text>
            </View>
          </View>

          {/* Upload Progress */}
          {isLoading && (
            <View style={styles.section}>
              <Text style={styles.sectionTitle}>Uploading...</Text>
              <View style={styles.progressContainer}>
                <ActivityIndicator size="large" color="#007AFF" />
                <Text style={styles.progressText}>
                  Uploading your video, please wait...
                </Text>
              </View>
            </View>
          )}

          {/* Upload Button */}
          <View style={styles.buttonContainer}>
            <TouchableOpacity
              style={[styles.uploadButton, isLoading && styles.uploadButtonDisabled]}
              onPress={handleUploadVideo}
              disabled={isLoading}
            >
              {isLoading ? (
                <ActivityIndicator color="#fff" />
              ) : (
                <Text style={styles.uploadButtonText}>Upload Video</Text>
              )}
            </TouchableOpacity>
          </View>
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8f9fa',
  },
  keyboardView: {
    flex: 1,
  },
  scrollView: {
    flex: 1,
    paddingHorizontal: 16,
  },
  section: {
    marginBottom: 24,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 12,
  },
  draftButton: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    backgroundColor: '#f0f0f0',
    borderRadius: 6,
  },
  draftButtonText: {
    fontSize: 14,
    color: '#007AFF',
    fontWeight: '500',
  },
  filePickerButton: {
    borderWidth: 2,
    borderColor: '#e1e5e9',
    borderStyle: 'dashed',
    borderRadius: 8,
    padding: 20,
    alignItems: 'center',
    backgroundColor: '#fafbfc',
  },
  filePickerError: {
    borderColor: '#ff3b30',
    backgroundColor: '#fff5f5',
  },
  filePickerContent: {
    alignItems: 'center',
  },
  filePickerIcon: {
    fontSize: 48,
    marginBottom: 8,
  },
  filePickerText: {
    fontSize: 16,
    fontWeight: '500',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  filePickerSubtext: {
    fontSize: 12,
    color: '#666',
    textAlign: 'center',
  },
  fileInfo: {
    alignItems: 'center',
  },
  fileName: {
    fontSize: 16,
    fontWeight: '500',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  fileSize: {
    fontSize: 14,
    color: '#666',
  },
  inputContainer: {
    marginBottom: 16,
  },
  inputLabel: {
    fontSize: 14,
    fontWeight: '500',
    color: '#1a1a1a',
    marginBottom: 6,
  },
  textInput: {
    borderWidth: 1,
    borderColor: '#e1e5e9',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 12,
    fontSize: 16,
    backgroundColor: '#fff',
    color: '#1a1a1a',
  },
  textInputMultiline: {
    height: 100,
    textAlignVertical: 'top',
  },
  textInputError: {
    borderColor: '#ff3b30',
    backgroundColor: '#fff5f5',
  },
  characterCount: {
    fontSize: 12,
    color: '#666',
    textAlign: 'right',
    marginTop: 4,
  },
  errorText: {
    fontSize: 12,
    color: '#ff3b30',
    marginTop: 4,
  },
  progressContainer: {
    alignItems: 'center',
    padding: 20,
  },
  progressText: {
    fontSize: 14,
    color: '#666',
    marginTop: 12,
    textAlign: 'center',
  },
  buttonContainer: {
    marginTop: 20,
    marginBottom: 40,
  },
  uploadButton: {
    backgroundColor: '#007AFF',
    borderRadius: 8,
    paddingVertical: 16,
    alignItems: 'center',
    justifyContent: 'center',
  },
  uploadButtonDisabled: {
    backgroundColor: '#ccc',
  },
  uploadButtonText: {
    fontSize: 16,
    fontWeight: '600',
    color: '#fff',
  },
});
