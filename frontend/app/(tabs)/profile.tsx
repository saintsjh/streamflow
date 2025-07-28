import React, { useState, useEffect } from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity, Alert, Dimensions, ActivityIndicator } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useAuth } from '@/contexts/AuthContext';
import Constants from 'expo-constants';

const { width } = Dimensions.get('window');

// Mock user data - replace with actual user data from auth context
const mockUser = {
  userName: 'john_doe',
  email: 'john.doe@example.com',
  createdAt: '2024-01-01T10:00:00Z',
};

// Mock uploaded videos - replace with actual API call
const mockUploadedVideos = [
  {
    id: '1',
    title: 'My First React Native Tutorial',
    thumbnail: null,
    createdAt: '2024-01-14T14:20:00Z',
    duration: '15:32',
    views: 1234,
  },
  {
    id: '2',
    title: 'Cooking Pasta at Home',
    thumbnail: null,
    createdAt: '2024-01-13T16:45:00Z',
    duration: '8:45',
    views: 856,
  },
  {
    id: '3',
    title: 'Guitar Practice Session',
    thumbnail: null,
    createdAt: '2024-01-12T12:30:00Z',
    duration: '22:15',
    views: 543,
  },
  {
    id: '4',
    title: 'Morning Workout Routine',
    thumbnail: null,
    createdAt: '2024-01-10T08:20:00Z',
    duration: '12:08',
    views: 2187,
  },
];

const formatDate = (dateString: string) => {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', { 
    year: 'numeric', 
    month: 'long', 
    day: 'numeric' 
  });
};

const formatTimeAgo = (dateString: string) => {
  const now = new Date();
  const createdAt = new Date(dateString);
  const diffInHours = Math.floor((now.getTime() - createdAt.getTime()) / (1000 * 60 * 60));
  
  if (diffInHours < 1) return 'Just now';
  if (diffInHours < 24) return `${diffInHours}h ago`;
  return `${Math.floor(diffInHours / 24)}d ago`;
};

const formatViews = (views: number) => {
  if (views >= 1000000) {
    return `${(views / 1000000).toFixed(1)}M views`;
  } else if (views >= 1000) {
    return `${(views / 1000).toFixed(1)}K views`;
  }
  return `${views} views`;
};

const VideoCard = ({ item }: { item: any }) => (
  <TouchableOpacity style={styles.videoCard}>
    <View style={styles.thumbnail}>
      <View style={styles.durationBadge}>
        <Text style={styles.durationText}>{item.duration}</Text>
      </View>
      <Text style={styles.placeholderText}>🎬</Text>
    </View>
    <View style={styles.videoInfo}>
      <Text style={styles.videoTitle} numberOfLines={2}>{item.title}</Text>
      <Text style={styles.videoMeta}>
        {formatViews(item.views)} • {formatTimeAgo(item.createdAt)}
      </Text>
    </View>
  </TouchableOpacity>
);

export default function ProfileScreen() {
  const { logout } = useAuth();
  const [user, setUser] = useState(mockUser);
  const [userVideos, setUserVideos] = useState(mockUploadedVideos);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadUserData();
  }, []);

  const loadUserData = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      // Load user profile and their videos in parallel
      const [userResponse, videosResponse] = await Promise.all([
        fetch(`${Constants.expoConfig?.extra?.apiBaseUrl || process.env.EXPO_PUBLIC_API_BASE_URL}/api/user/me`, {
          headers: { 'Authorization': `Bearer ${token}` },
        }),
        fetch(`${Constants.expoConfig?.extra?.apiBaseUrl || process.env.EXPO_PUBLIC_API_BASE_URL}/api/video/list?page=1&limit=50`, {
          headers: { 'Authorization': `Bearer ${token}` },
        }),
      ]);

      let userID = null;
      if (userResponse.ok) {
        const userData = await userResponse.json();
        userID = userData.ID;
        setUser({
          userName: userData.userName || userData.email.split('@')[0],
          email: userData.email,
          createdAt: userData.createdAt,
        });
      }

      if (videosResponse.ok && userID) {
        const videosData = await videosResponse.json();
        // Filter to only show user's own videos and convert to frontend format
        const formattedVideos = videosData
          .filter((video: any) => video.UserID === userID)
          .map((video: any) => ({
            id: video.ID,
            title: video.Title,
            thumbnail: null,
            createdAt: video.CreatedAt,
            duration: Math.floor(video.Metadata?.Duration || 0),
            views: video.ViewCount || 0,
          }));
        setUserVideos(formattedVideos);
      }
    } catch (error) {
      console.error('Error loading user data:', error);
      Alert.alert('Error', 'Failed to load profile data. Please try again.');
    } finally {
      setIsLoading(false);
    }
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

  const handleEditProfile = () => {
    // TODO: Navigate to edit profile screen
    Alert.alert('Edit Profile', 'This feature will be implemented soon!');
  };

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <ScrollView style={styles.scrollView} showsVerticalScrollIndicator={false}>
        {/* User Info Section */}
        <View style={styles.userSection}>
          <View style={styles.avatarContainer}>
            <View style={styles.avatar}>
              <Text style={styles.avatarText}>
                {user.userName.charAt(0).toUpperCase()}
              </Text>
            </View>
          </View>
          
          <View style={styles.userInfo}>
            <Text style={styles.userName}>@{user.userName}</Text>
            <Text style={styles.userEmail}>{user.email}</Text>
            <Text style={styles.memberSince}>
              Member since {formatDate(user.createdAt)}
            </Text>
          </View>
          
          <View style={styles.actionButtons}>
            <TouchableOpacity style={styles.editButton} onPress={handleEditProfile}>
              <Text style={styles.editButtonText}>Edit Profile</Text>
            </TouchableOpacity>
            
            <TouchableOpacity style={styles.logoutButton} onPress={handleLogout}>
              <Text style={styles.logoutButtonText}>Logout</Text>
            </TouchableOpacity>
          </View>
        </View>

        {/* Stats Section */}
        <View style={styles.statsSection}>
          <View style={styles.statItem}>
            <Text style={styles.statNumber}>{userVideos.length}</Text>
            <Text style={styles.statLabel}>Videos</Text>
          </View>
          <View style={styles.statDivider} />
          <View style={styles.statItem}>
            <Text style={styles.statNumber}>
              {userVideos.reduce((total, video) => total + video.views, 0).toLocaleString()}
            </Text>
            <Text style={styles.statLabel}>Total Views</Text>
          </View>
        </View>

        {/* Uploaded Videos Section */}
        <View style={styles.videosSection}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>📹 My Videos</Text>
            <TouchableOpacity>
              <Text style={styles.seeAllText}>Manage</Text>
            </TouchableOpacity>
          </View>
          
          <View style={styles.videosGrid}>
            {isLoading ? (
              <ActivityIndicator size="large" color="#007AFF" />
            ) : userVideos.length > 0 ? (
              userVideos.map((item) => (
                <View key={item.id} style={styles.videoCardWrapper}>
                  <VideoCard item={item} />
                </View>
              ))
            ) : (
              <Text style={styles.noVideosText}>No videos uploaded yet</Text>
            )}
          </View>
        </View>
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
  userSection: {
    backgroundColor: '#fff',
    paddingVertical: 24,
    paddingHorizontal: 20,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: '#e1e5e9',
  },
  avatarContainer: {
    marginBottom: 16,
  },
  avatar: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: '#007AFF',
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatarText: {
    fontSize: 32,
    fontWeight: 'bold',
    color: '#fff',
  },
  userInfo: {
    alignItems: 'center',
    marginBottom: 20,
  },
  userName: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  userEmail: {
    fontSize: 16,
    color: '#666',
    marginBottom: 4,
  },
  memberSince: {
    fontSize: 14,
    color: '#888',
  },
  actionButtons: {
    flexDirection: 'row',
    gap: 12,
  },
  editButton: {
    backgroundColor: '#007AFF',
    paddingVertical: 10,
    paddingHorizontal: 20,
    borderRadius: 8,
  },
  editButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 16,
  },
  logoutButton: {
    backgroundColor: '#ff3b30',
    paddingVertical: 10,
    paddingHorizontal: 20,
    borderRadius: 8,
  },
  logoutButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 16,
  },
  statsSection: {
    backgroundColor: '#fff',
    paddingVertical: 20,
    paddingHorizontal: 20,
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    marginTop: 16,
    marginHorizontal: 16,
    borderRadius: 12,
  },
  statItem: {
    alignItems: 'center',
    flex: 1,
  },
  statNumber: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  statLabel: {
    fontSize: 14,
    color: '#666',
  },
  statDivider: {
    width: 1,
    height: 40,
    backgroundColor: '#e1e5e9',
    marginHorizontal: 20,
  },
  videosSection: {
    marginTop: 16,
    paddingBottom: 20,
  },
  sectionHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 20,
    marginBottom: 16,
  },
  sectionTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
  },
  seeAllText: {
    fontSize: 16,
    color: '#007AFF',
    fontWeight: '500',
  },
  videosGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    paddingHorizontal: 10,
  },
  videoCardWrapper: {
    width: '50%',
    paddingHorizontal: 10,
  },
  videoCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    marginBottom: 16,
    shadowColor: '#000',
    shadowOffset: {
      width: 0,
      height: 2,
    },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  thumbnail: {
    width: '100%',
    height: width * 0.225, // 16:9 aspect ratio
    backgroundColor: '#e1e5e9',
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    position: 'relative',
  },
  placeholderText: {
    fontSize: 32,
    opacity: 0.5,
  },
  durationBadge: {
    position: 'absolute',
    bottom: 8,
    right: 8,
    backgroundColor: 'rgba(0,0,0,0.8)',
    paddingHorizontal: 6,
    paddingVertical: 2,
    borderRadius: 4,
  },
  durationText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: '500',
  },
  videoInfo: {
    padding: 12,
  },
  videoTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 4,
    lineHeight: 18,
  },
  videoMeta: {
    fontSize: 12,
    color: '#666',
  },
  noVideosText: {
    textAlign: 'center',
    color: '#666',
    fontSize: 16,
    marginTop: 40,
  },
}); 