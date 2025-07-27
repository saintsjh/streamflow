import React, { useState, useEffect } from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity, Switch, Alert } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useAuth } from '@/contexts/AuthContext';
import BackHeader from '@/components/BackHeader';

// Mock user settings - replace with actual API calls and AsyncStorage
const defaultSettings = {
  notifications: {
    pushNotifications: true,
    emailNotifications: false,
    streamAlerts: true,
    commentNotifications: true,
    followerNotifications: true,
  },
  privacy: {
    profileVisibility: 'public', // public, private, friends
    showActivity: true,
    showFollowers: true,
    allowComments: true,
    allowDirectMessages: true,
  },
  streaming: {
    defaultQuality: 'HD', // HD, FHD, 4K
    autoRecord: true,
    chatModeration: 'medium', // off, low, medium, high
    allowGifts: true,
  },
  app: {
    theme: 'auto', // light, dark, auto
    autoplay: true,
    dataUsage: 'wifi', // wifi, always, never
    language: 'en',
    downloadQuality: 'medium',
  },
  account: {
    twoFactorAuth: false,
    loginAlerts: true,
    sessionTimeout: 30, // days
  }
};

type SettingItemProps = {
  title: string;
  subtitle?: string;
  onPress?: () => void;
  rightElement?: React.ReactNode;
  showArrow?: boolean;
};

const SettingItem = ({ title, subtitle, onPress, rightElement, showArrow = false }: SettingItemProps) => (
  <TouchableOpacity 
    style={styles.settingItem} 
    onPress={onPress}
    disabled={!onPress}
  >
    <View style={styles.settingContent}>
      <Text style={styles.settingTitle}>{title}</Text>
      {subtitle && <Text style={styles.settingSubtitle}>{subtitle}</Text>}
    </View>
    <View style={styles.settingRight}>
      {rightElement}
      {showArrow && <Text style={styles.arrow}>›</Text>}
    </View>
  </TouchableOpacity>
);

type SettingSectionProps = {
  title: string;
  children: React.ReactNode;
};

const SettingSection = ({ title, children }: SettingSectionProps) => (
  <View style={styles.section}>
    <Text style={styles.sectionTitle}>{title}</Text>
    <View style={styles.sectionContent}>
      {children}
    </View>
  </View>
);

export default function SettingsScreen() {
  const { logout } = useAuth();
  const [settings, setSettings] = useState(defaultSettings);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      const stored = await AsyncStorage.getItem('userSettings');
      if (stored) {
        setSettings({ ...defaultSettings, ...JSON.parse(stored) });
      }
    } catch (error) {
      console.error('Error loading settings:', error);
    }
  };

  const saveSettings = async (newSettings: typeof defaultSettings) => {
    try {
      setSettings(newSettings);
      await AsyncStorage.setItem('userSettings', JSON.stringify(newSettings));
    } catch (error) {
      console.error('Error saving settings:', error);
    }
  };

  const updateSetting = (category: keyof typeof defaultSettings, key: string, value: any) => {
    const newSettings = {
      ...settings,
      [category]: {
        ...settings[category],
        [key]: value,
      },
    };
    saveSettings(newSettings);
  };

  const handleLogout = () => {
    Alert.alert(
      'Logout',
      'Are you sure you want to logout?',
      [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Logout', style: 'destructive', onPress: logout },
      ]
    );
  };

  const handleDeleteAccount = () => {
    Alert.alert(
      'Delete Account',
      'This action cannot be undone. All your content will be permanently deleted.',
      [
        { text: 'Cancel', style: 'cancel' },
        { 
          text: 'Delete', 
          style: 'destructive', 
          onPress: () => {
            Alert.alert('Feature Coming Soon', 'Account deletion will be available in a future update.');
          }
        },
      ]
    );
  };

  const handleExportData = () => {
    Alert.alert('Export Data', 'Your data export will be ready within 24 hours and sent to your email.');
  };

  const handleClearCache = async () => {
    try {
      // Clear recently viewed and other cached data
      await AsyncStorage.removeItem('recentlyViewed');
      Alert.alert('Success', 'Cache cleared successfully');
    } catch (error) {
      Alert.alert('Error', 'Failed to clear cache');
    }
  };

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <BackHeader 
        title="Settings" 
        subtitle="Customize your app experience"
        rightElement={
          <TouchableOpacity style={styles.saveButton}>
            <Text style={styles.saveButtonText}>Save</Text>
          </TouchableOpacity>
        }
      />
      <ScrollView style={styles.scrollView} showsVerticalScrollIndicator={false}>
        <SettingSection title="👤 Account">
          <SettingItem
            title="Two-Factor Authentication"
            subtitle="Add extra security to your account"
            rightElement={
              <Switch
                value={settings.account.twoFactorAuth}
                onValueChange={(value) => updateSetting('account', 'twoFactorAuth', value)}
              />
            }
          />
          <SettingItem
            title="Login Alerts"
            subtitle="Get notified of new logins"
            rightElement={
              <Switch
                value={settings.account.loginAlerts}
                onValueChange={(value) => updateSetting('account', 'loginAlerts', value)}
              />
            }
          />
        </SettingSection>
        <SettingSection title="🔔 Notifications">
          <SettingItem
            title="Push Notifications"
            subtitle="Receive notifications on your device"
            rightElement={
              <Switch
                value={settings.notifications.pushNotifications}
                onValueChange={(value) => updateSetting('notifications', 'pushNotifications', value)}
              />
            }
          />
          <SettingItem
            title="Email Notifications"
            subtitle="Receive updates via email"
            rightElement={
              <Switch
                value={settings.notifications.emailNotifications}
                onValueChange={(value) => updateSetting('notifications', 'emailNotifications', value)}
              />
            }
          />
          <SettingItem
            title="Stream Alerts"
            subtitle="Get notified when followed creators go live"
            rightElement={
              <Switch
                value={settings.notifications.streamAlerts}
                onValueChange={(value) => updateSetting('notifications', 'streamAlerts', value)}
              />
            }
          />
          <SettingItem
            title="Comment Notifications"
            subtitle="Get notified of comments on your content"
            rightElement={
              <Switch
                value={settings.notifications.commentNotifications}
                onValueChange={(value) => updateSetting('notifications', 'commentNotifications', value)}
              />
            }
          />
          <SettingItem
            title="New Follower Notifications"
            subtitle="Get notified when someone follows you"
            rightElement={
              <Switch
                value={settings.notifications.followerNotifications}
                onValueChange={(value) => updateSetting('notifications', 'followerNotifications', value)}
              />
            }
          />
        </SettingSection>
        <SettingSection title="🔒 Privacy">
          <SettingItem
            title="Profile Visibility"
            subtitle={`Currently: ${settings.privacy.profileVisibility}`}
            onPress={() => {
              Alert.alert(
                'Profile Visibility',
                'Choose who can see your profile',
                [
                  { text: 'Public', onPress: () => updateSetting('privacy', 'profileVisibility', 'public') },
                  { text: 'Private', onPress: () => updateSetting('privacy', 'profileVisibility', 'private') },
                  { text: 'Cancel', style: 'cancel' },
                ]
              );
            }}
            showArrow
          />
          <SettingItem
            title="Show Activity Status"
            subtitle="Let others see when you're online"
            rightElement={
              <Switch
                value={settings.privacy.showActivity}
                onValueChange={(value) => updateSetting('privacy', 'showActivity', value)}
              />
            }
          />
          <SettingItem
            title="Allow Comments"
            subtitle="Let others comment on your content"
            rightElement={
              <Switch
                value={settings.privacy.allowComments}
                onValueChange={(value) => updateSetting('privacy', 'allowComments', value)}
              />
            }
          />
          <SettingItem
            title="Allow Direct Messages"
            subtitle="Receive messages from other users"
            rightElement={
              <Switch
                value={settings.privacy.allowDirectMessages}
                onValueChange={(value) => updateSetting('privacy', 'allowDirectMessages', value)}
              />
            }
          />
        </SettingSection>
        <SettingSection title="🔴 Streaming">
          <SettingItem
            title="Default Stream Quality"
            subtitle={`Currently: ${settings.streaming.defaultQuality}`}
            onPress={() => {
              Alert.alert(
                'Stream Quality',
                'Choose your default streaming quality',
                [
                  { text: 'HD (720p)', onPress: () => updateSetting('streaming', 'defaultQuality', 'HD') },
                  { text: 'FHD (1080p)', onPress: () => updateSetting('streaming', 'defaultQuality', 'FHD') },
                  { text: '4K (2160p)', onPress: () => updateSetting('streaming', 'defaultQuality', '4K') },
                  { text: 'Cancel', style: 'cancel' },
                ]
              );
            }}
            showArrow
          />
          <SettingItem
            title="Auto-Record Streams"
            subtitle="Automatically save your live streams"
            rightElement={
              <Switch
                value={settings.streaming.autoRecord}
                onValueChange={(value) => updateSetting('streaming', 'autoRecord', value)}
              />
            }
          />
          <SettingItem
            title="Chat Moderation"
            subtitle={`Currently: ${settings.streaming.chatModeration}`}
            onPress={() => {
              Alert.alert(
                'Chat Moderation',
                'Set automatic moderation level',
                [
                  { text: 'Off', onPress: () => updateSetting('streaming', 'chatModeration', 'off') },
                  { text: 'Low', onPress: () => updateSetting('streaming', 'chatModeration', 'low') },
                  { text: 'Medium', onPress: () => updateSetting('streaming', 'chatModeration', 'medium') },
                  { text: 'High', onPress: () => updateSetting('streaming', 'chatModeration', 'high') },
                  { text: 'Cancel', style: 'cancel' },
                ]
              );
            }}
            showArrow
          />
        </SettingSection>
        <SettingSection title="📱 App Preferences">
          <SettingItem
            title="Theme"
            subtitle={`Currently: ${settings.app.theme}`}
            onPress={() => {
              Alert.alert(
                'App Theme',
                'Choose your preferred theme',
                [
                  { text: 'Light', onPress: () => updateSetting('app', 'theme', 'light') },
                  { text: 'Dark', onPress: () => updateSetting('app', 'theme', 'dark') },
                  { text: 'Auto', onPress: () => updateSetting('app', 'theme', 'auto') },
                  { text: 'Cancel', style: 'cancel' },
                ]
              );
            }}
            showArrow
          />
          <SettingItem
            title="Autoplay Videos"
            subtitle="Automatically play videos when scrolling"
            rightElement={
              <Switch
                value={settings.app.autoplay}
                onValueChange={(value) => updateSetting('app', 'autoplay', value)}
              />
            }
          />
          <SettingItem
            title="Data Usage"
            subtitle={`Video streaming: ${settings.app.dataUsage}`}
            onPress={() => {
              Alert.alert(
                'Data Usage',
                'When to stream high-quality video',
                [
                  { text: 'WiFi Only', onPress: () => updateSetting('app', 'dataUsage', 'wifi') },
                  { text: 'Always', onPress: () => updateSetting('app', 'dataUsage', 'always') },
                  { text: 'Never', onPress: () => updateSetting('app', 'dataUsage', 'never') },
                  { text: 'Cancel', style: 'cancel' },
                ]
              );
            }}
            showArrow
          />
          <SettingItem
            title="Clear Cache"
            subtitle="Clear app cache and temporary files"
            onPress={handleClearCache}
            showArrow
          />
        </SettingSection>
        <SettingSection title="ℹ️ Support & Information">
          <SettingItem
            title="Help & Support"
            subtitle="Get help with using the app"
            onPress={() => Alert.alert('Help', 'This will open the help center')}
            showArrow
          />
          <SettingItem
            title="Terms of Service"
            subtitle="Read our terms and conditions"
            onPress={() => Alert.alert('Terms', 'This will open terms of service')}
            showArrow
          />
          <SettingItem
            title="Privacy Policy"
            subtitle="Read our privacy policy"
            onPress={() => Alert.alert('Privacy', 'This will open privacy policy')}
            showArrow
          />
          <SettingItem
            title="About"
            subtitle="App version and information"
            onPress={() => Alert.alert('About', 'VideoStream v1.0.0\nBuilt with React Native & Expo')}
            showArrow
          />
        </SettingSection>
        <SettingSection title="🗂️ Data Management">
          <SettingItem
            title="Export My Data"
            subtitle="Download a copy of your data"
            onPress={handleExportData}
            showArrow
          />
          <SettingItem
            title="Delete Account"
            subtitle="Permanently delete your account and data"
            onPress={handleDeleteAccount}
            showArrow
          />
        </SettingSection>
        <View style={styles.logoutSection}>
          <TouchableOpacity style={styles.logoutButton} onPress={handleLogout}>
            <Text style={styles.logoutButtonText}>Logout</Text>
          </TouchableOpacity>
        </View>
        <View style={styles.bottomPadding} />
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8f9fa',
  },
  scrollView: {
    flex: 1,
  },
  section: {
    marginTop: 24,
  },
  sectionTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
    paddingHorizontal: 20,
    marginBottom: 12,
  },
  sectionContent: {
    backgroundColor: '#fff',
    marginHorizontal: 16,
    borderRadius: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  settingItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 16,
    paddingHorizontal: 16,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  settingContent: {
    flex: 1,
    marginRight: 12,
  },
  settingTitle: {
    fontSize: 16,
    fontWeight: '500',
    color: '#1a1a1a',
    marginBottom: 2,
  },
  settingSubtitle: {
    fontSize: 14,
    color: '#666',
  },
  settingRight: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  arrow: {
    fontSize: 20,
    color: '#ccc',
    marginLeft: 8,
  },
  logoutSection: {
    marginTop: 32,
    paddingHorizontal: 20,
  },
  logoutButton: {
    backgroundColor: '#ff3b30',
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  logoutButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  bottomPadding: {
    height: 32,
  },
  saveButton: {
    backgroundColor: '#007AFF',
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 16,
  },
  saveButtonText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
  },
}); 