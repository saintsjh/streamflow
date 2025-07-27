import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Switch,
  Alert,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router } from 'expo-router';
import AsyncStorage from '@react-native-async-storage/async-storage';
import BackHeader from '@/components/BackHeader';
import { useAuth } from '@/contexts/AuthContext';

// Types based on backend structs
type StreamConfigurationData = {
  title: string;
  description: string;
  // Additional frontend-only configuration options
  enableChat: boolean;
  enableRecording: boolean;
  isPrivate: boolean;
  category: string;
  quality: 'LOW' | 'MEDIUM' | 'HIGH' | 'ULTRA';
  maxViewers?: number;
};

type StreamCategory = {
  id: string;
  name: string;
  emoji: string;
};

const streamCategories: StreamCategory[] = [
  { id: 'gaming', name: 'Gaming', emoji: '🎮' },
  { id: 'music', name: 'Music', emoji: '🎵' },
  { id: 'education', name: 'Education', emoji: '📚' },
  { id: 'technology', name: 'Technology', emoji: '💻' },
  { id: 'lifestyle', name: 'Lifestyle', emoji: '✨' },
  { id: 'art', name: 'Art & Design', emoji: '🎨' },
  { id: 'business', name: 'Business', emoji: '💼' },
  { id: 'other', name: 'Other', emoji: '📝' },
];

const qualityOptions = [
  { value: 'LOW', label: '720p (Low)', description: 'Good for slower connections' },
  { value: 'MEDIUM', label: '1080p (Medium)', description: 'Balanced quality and performance' },
  { value: 'HIGH', label: '1440p (High)', description: 'High quality streaming' },
  { value: 'ULTRA', label: '4K (Ultra)', description: 'Maximum quality (requires fast internet)' },
] as const;

export default function StreamConfigurationScreen() {
  const { logout } = useAuth();
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  
  const [config, setConfig] = useState<StreamConfigurationData>({
    title: '',
    description: '',
    enableChat: true,
    enableRecording: false,
    isPrivate: false,
    category: 'other',
    quality: 'MEDIUM',
    maxViewers: undefined,
  });

  // Validation rules
  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!config.title.trim()) {
      newErrors.title = 'Stream title is required';
    } else if (config.title.length < 3) {
      newErrors.title = 'Title must be at least 3 characters';
    } else if (config.title.length > 100) {
      newErrors.title = 'Title must be less than 100 characters';
    }

    if (config.description.length > 500) {
      newErrors.description = 'Description must be less than 500 characters';
    }

    if (config.maxViewers && (config.maxViewers < 1 || config.maxViewers > 10000)) {
      newErrors.maxViewers = 'Max viewers must be between 1 and 10,000';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleStartStream = async () => {
    if (!validateForm()) {
      Alert.alert('Validation Error', 'Please fix the errors before starting your stream.');
      return;
    }

    setIsLoading(true);
    
    try {
      // Prepare data for backend StartStreamRequest
      const streamRequest = {
        title: config.title.trim(),
        description: config.description.trim(),
      };

      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      const response = await fetch('http://localhost:8080/api/livestream/start', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(streamRequest),
      });

      const result = await response.json();
      
      if (!response.ok) {
        throw new Error(result.error || 'Failed to start stream');
      }
      
              Alert.alert(
          'Stream Starting! 🎉',
          `Your stream "${config.title}" is being prepared. You'll be redirected to the live streaming interface.`,
          [
            {
              text: 'Start Broadcasting',
              onPress: () => {
                router.push(`/screens/livestream?id=${result.ID}`);
              },
            },
          ]
        );
    } catch (error) {
      console.error('Error starting stream:', error);
      Alert.alert(
        'Stream Error',
        'Failed to start your stream. Please check your connection and try again.',
        [{ text: 'OK' }]
      );
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveDraft = async () => {
    try {
      // Save configuration as draft
      console.log('Saving draft configuration:', config);
      Alert.alert('Draft Saved', 'Your stream configuration has been saved as a draft.');
    } catch (error) {
      console.error('Error saving draft:', error);
    }
  };

  const CategorySelector = () => (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>Category</Text>
      <View style={styles.categoryGrid}>
        {streamCategories.map((category) => (
          <TouchableOpacity
            key={category.id}
            style={[
              styles.categoryButton,
              config.category === category.id && styles.categoryButtonSelected,
            ]}
            onPress={() => setConfig({ ...config, category: category.id })}
          >
            <Text style={styles.categoryEmoji}>{category.emoji}</Text>
            <Text
              style={[
                styles.categoryText,
                config.category === category.id && styles.categoryTextSelected,
              ]}
            >
              {category.name}
            </Text>
          </TouchableOpacity>
        ))}
      </View>
    </View>
  );

  const QualitySelector = () => (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>Stream Quality</Text>
      {qualityOptions.map((option) => (
        <TouchableOpacity
          key={option.value}
          style={[
            styles.qualityOption,
            config.quality === option.value && styles.qualityOptionSelected,
          ]}
          onPress={() => setConfig({ ...config, quality: option.value })}
        >
          <View style={styles.qualityOptionContent}>
            <View style={styles.qualityOptionLeft}>
              <Text
                style={[
                  styles.qualityLabel,
                  config.quality === option.value && styles.qualityLabelSelected,
                ]}
              >
                {option.label}
              </Text>
              <Text style={styles.qualityDescription}>{option.description}</Text>
            </View>
            <View
              style={[
                styles.radioButton,
                config.quality === option.value && styles.radioButtonSelected,
              ]}
            />
          </View>
        </TouchableOpacity>
      ))}
    </View>
  );

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <BackHeader
        title="Stream Setup"
        subtitle="Configure your live stream"
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
          {/* Basic Information */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Basic Information</Text>
            
            <View style={styles.inputContainer}>
              <Text style={styles.inputLabel}>Stream Title *</Text>
              <TextInput
                style={[styles.textInput, errors.title && styles.textInputError]}
                value={config.title}
                onChangeText={(text) => {
                  setConfig({ ...config, title: text });
                  if (errors.title) {
                    setErrors({ ...errors, title: '' });
                  }
                }}
                placeholder="Enter your stream title"
                placeholderTextColor="#999"
                maxLength={100}
              />
              {errors.title && <Text style={styles.errorText}>{errors.title}</Text>}
              <Text style={styles.characterCount}>{config.title.length}/100</Text>
            </View>

            <View style={styles.inputContainer}>
              <Text style={styles.inputLabel}>Description</Text>
              <TextInput
                style={[styles.textAreaInput, errors.description && styles.textInputError]}
                value={config.description}
                onChangeText={(text) => {
                  setConfig({ ...config, description: text });
                  if (errors.description) {
                    setErrors({ ...errors, description: '' });
                  }
                }}
                placeholder="Tell viewers what your stream is about..."
                placeholderTextColor="#999"
                multiline
                numberOfLines={4}
                textAlignVertical="top"
                maxLength={500}
              />
              {errors.description && <Text style={styles.errorText}>{errors.description}</Text>}
              <Text style={styles.characterCount}>{config.description.length}/500</Text>
            </View>
          </View>

          <CategorySelector />
          <QualitySelector />

          {/* Advanced Settings */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Stream Settings</Text>
            
            <View style={styles.settingRow}>
              <View style={styles.settingInfo}>
                <Text style={styles.settingLabel}>Enable Chat</Text>
                <Text style={styles.settingDescription}>Allow viewers to chat during your stream</Text>
              </View>
              <Switch
                value={config.enableChat}
                onValueChange={(value) => setConfig({ ...config, enableChat: value })}
                trackColor={{ false: '#e1e5e9', true: '#007AFF' }}
                thumbColor={config.enableChat ? '#fff' : '#f4f3f4'}
              />
            </View>

            <View style={styles.settingRow}>
              <View style={styles.settingInfo}>
                <Text style={styles.settingLabel}>Record Stream</Text>
                <Text style={styles.settingDescription}>Save a recording of your stream</Text>
              </View>
              <Switch
                value={config.enableRecording}
                onValueChange={(value) => setConfig({ ...config, enableRecording: value })}
                trackColor={{ false: '#e1e5e9', true: '#007AFF' }}
                thumbColor={config.enableRecording ? '#fff' : '#f4f3f4'}
              />
            </View>

            <View style={styles.settingRow}>
              <View style={styles.settingInfo}>
                <Text style={styles.settingLabel}>Private Stream</Text>
                <Text style={styles.settingDescription}>Only you can invite viewers</Text>
              </View>
              <Switch
                value={config.isPrivate}
                onValueChange={(value) => setConfig({ ...config, isPrivate: value })}
                trackColor={{ false: '#e1e5e9', true: '#007AFF' }}
                thumbColor={config.isPrivate ? '#fff' : '#f4f3f4'}
              />
            </View>

            <View style={styles.inputContainer}>
              <Text style={styles.inputLabel}>Max Viewers (Optional)</Text>
              <TextInput
                style={[styles.textInput, errors.maxViewers && styles.textInputError]}
                value={config.maxViewers?.toString() || ''}
                onChangeText={(text) => {
                  const num = text ? parseInt(text) : undefined;
                  setConfig({ ...config, maxViewers: num });
                  if (errors.maxViewers) {
                    setErrors({ ...errors, maxViewers: '' });
                  }
                }}
                placeholder="Unlimited"
                placeholderTextColor="#999"
                keyboardType="numeric"
              />
              {errors.maxViewers && <Text style={styles.errorText}>{errors.maxViewers}</Text>}
              <Text style={styles.characterCount}>Leave empty for unlimited viewers</Text>
            </View>
          </View>

          {/* Bottom Padding */}
          <View style={styles.bottomPadding} />
        </ScrollView>

        {/* Action Buttons */}
        <View style={styles.actionButtons}>
          <TouchableOpacity
            style={styles.cancelButton}
            onPress={() => router.back()}
            disabled={isLoading}
          >
            <Text style={styles.cancelButtonText}>Cancel</Text>
          </TouchableOpacity>

          <TouchableOpacity
            style={[styles.startButton, isLoading && styles.startButtonDisabled]}
            onPress={handleStartStream}
            disabled={isLoading}
          >
            {isLoading ? (
              <ActivityIndicator color="#fff" size="small" />
            ) : (
              <Text style={styles.startButtonText}>🔴 Start Stream</Text>
            )}
          </TouchableOpacity>
        </View>
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
  },
  section: {
    backgroundColor: '#fff',
    marginTop: 16,
    paddingHorizontal: 20,
    paddingVertical: 20,
  },
  sectionTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 16,
  },
  inputContainer: {
    marginBottom: 20,
  },
  inputLabel: {
    fontSize: 16,
    fontWeight: '500',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  textInput: {
    borderWidth: 1,
    borderColor: '#e1e5e9',
    borderRadius: 12,
    paddingHorizontal: 16,
    paddingVertical: 12,
    fontSize: 16,
    color: '#1a1a1a',
    backgroundColor: '#fff',
  },
  textAreaInput: {
    borderWidth: 1,
    borderColor: '#e1e5e9',
    borderRadius: 12,
    paddingHorizontal: 16,
    paddingVertical: 12,
    fontSize: 16,
    color: '#1a1a1a',
    backgroundColor: '#fff',
    minHeight: 100,
  },
  textInputError: {
    borderColor: '#ff3b30',
  },
  errorText: {
    color: '#ff3b30',
    fontSize: 14,
    marginTop: 4,
  },
  characterCount: {
    fontSize: 12,
    color: '#666',
    textAlign: 'right',
    marginTop: 4,
  },
  categoryGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 12,
  },
  categoryButton: {
    backgroundColor: '#f1f3f4',
    borderRadius: 12,
    paddingVertical: 12,
    paddingHorizontal: 16,
    alignItems: 'center',
    minWidth: 80,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  categoryButtonSelected: {
    backgroundColor: '#e3f2fd',
    borderColor: '#007AFF',
  },
  categoryEmoji: {
    fontSize: 20,
    marginBottom: 4,
  },
  categoryText: {
    fontSize: 12,
    color: '#666',
    fontWeight: '500',
  },
  categoryTextSelected: {
    color: '#007AFF',
    fontWeight: '600',
  },
  qualityOption: {
    backgroundColor: '#f8f9fa',
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  qualityOptionSelected: {
    backgroundColor: '#e3f2fd',
    borderColor: '#007AFF',
  },
  qualityOptionContent: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  qualityOptionLeft: {
    flex: 1,
  },
  qualityLabel: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  qualityLabelSelected: {
    color: '#007AFF',
  },
  qualityDescription: {
    fontSize: 14,
    color: '#666',
  },
  radioButton: {
    width: 20,
    height: 20,
    borderRadius: 10,
    borderWidth: 2,
    borderColor: '#ccc',
    marginLeft: 12,
  },
  radioButtonSelected: {
    borderColor: '#007AFF',
    backgroundColor: '#007AFF',
  },
  settingRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  settingInfo: {
    flex: 1,
    marginRight: 16,
  },
  settingLabel: {
    fontSize: 16,
    fontWeight: '500',
    color: '#1a1a1a',
    marginBottom: 2,
  },
  settingDescription: {
    fontSize: 14,
    color: '#666',
  },
  actionButtons: {
    flexDirection: 'row',
    paddingHorizontal: 20,
    paddingVertical: 16,
    backgroundColor: '#fff',
    borderTopWidth: 1,
    borderTopColor: '#e1e5e9',
    gap: 12,
  },
  cancelButton: {
    flex: 1,
    backgroundColor: '#f1f3f4',
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: 'center',
  },
  cancelButtonText: {
    fontSize: 16,
    fontWeight: '600',
    color: '#666',
  },
  startButton: {
    flex: 2,
    backgroundColor: '#ff3b30',
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: 'center',
  },
  startButtonDisabled: {
    backgroundColor: '#ccc',
  },
  startButtonText: {
    fontSize: 16,
    fontWeight: '600',
    color: '#fff',
  },
  draftButton: {
    backgroundColor: '#007AFF',
    borderRadius: 8,
    paddingVertical: 6,
    paddingHorizontal: 12,
  },
  draftButtonText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
  },
  bottomPadding: {
    height: 20,
  },
});
