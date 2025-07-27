import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity, FlatList, Dimensions, Alert, ActivityIndicator } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useAuth } from '@/contexts/AuthContext';
import { router } from 'expo-router';

const { width } = Dimensions.get('window');

// Mock data for recently viewed content (stored locally)
const mockRecentlyViewed = [
  {
    id: '1',
    title: 'React Native Basics',
    type: 'video',
    duration: '12:45',
    viewedAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(), // 2 hours ago
  },
  {
    id: '2',
    title: 'Live Coding Session',
    type: 'stream',
    duration: 'LIVE',
    viewedAt: new Date(Date.now() - 5 * 60 * 60 * 1000).toISOString(), // 5 hours ago
  },
];

// Mock trending content
const mockTrendingContent = [
  {
    id: '1',
    title: 'JavaScript Advanced Tips',
    views: 15420,
    trending: true,
    duration: '18:30',
  },
  {
    id: '2',
    title: 'Mobile App Design Trends',
    views: 8934,
    trending: true,
    duration: '25:15',
  },
];

const formatViews = (views: number) => {
  if (views >= 1000000) return `${(views / 1000000).toFixed(1)}M`;
  if (views >= 1000) return `${(views / 1000).toFixed(1)}K`;
  return views.toString();
};

const formatTimeAgo = (dateString: string) => {
  const now = new Date();
  const viewedAt = new Date(dateString);
  const diffInHours = Math.floor((now.getTime() - viewedAt.getTime()) / (1000 * 60 * 60));
  
  if (diffInHours < 1) return 'Just now';
  if (diffInHours < 24) return `${diffInHours}h ago`;
  return `${Math.floor(diffInHours / 24)}d ago`;
};

const RecentlyViewedCard = ({ item }: { item: any }) => (
  <TouchableOpacity style={styles.recentCard}>
    <View style={styles.recentCardThumbnail}>
      <Text style={styles.placeholderIcon}>
        {item.type === 'stream' ? '📹' : '🎬'}
      </Text>
      {item.type === 'stream' && item.duration === 'LIVE' ? (
        <View style={styles.liveIndicator}>
          <Text style={styles.liveText}>LIVE</Text>
        </View>
      ) : (
        <View style={styles.durationBadge}>
          <Text style={styles.durationText}>{item.duration}</Text>
        </View>
      )}
    </View>
    <View style={styles.recentCardInfo}>
      <Text style={styles.recentCardTitle} numberOfLines={2}>{item.title}</Text>
      <Text style={styles.recentCardMeta}>Watched {formatTimeAgo(item.viewedAt)}</Text>
    </View>
  </TouchableOpacity>
);

const TrendingCard = ({ item }: { item: any }) => (
  <TouchableOpacity style={styles.trendingCard}>
    <View style={styles.trendingCardThumbnail}>
      <Text style={styles.placeholderIcon}>🔥</Text>
      <View style={styles.durationBadge}>
        <Text style={styles.durationText}>{item.duration}</Text>
      </View>
    </View>
    <View style={styles.trendingCardInfo}>
      <Text style={styles.trendingCardTitle} numberOfLines={2}>{item.title}</Text>
      <Text style={styles.trendingCardMeta}>{formatViews(item.views)} views</Text>
    </View>
  </TouchableOpacity>
);

export default function HomeScreen() {
  const { logout } = useAuth();
  const [currentTime, setCurrentTime] = useState(new Date());
  const [recentlyViewed, setRecentlyViewed] = useState(mockRecentlyViewed);
  const [trendingContent, setTrendingContent] = useState(mockTrendingContent);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Update time every minute for dynamic greeting
    const timer = setInterval(() => {
      setCurrentTime(new Date());
    }, 60000);

    // Load recently viewed content from AsyncStorage
    loadRecentlyViewed();
    
    // Load trending content from API
    loadTrendingContent();

    return () => clearInterval(timer);
  }, []);

  const loadRecentlyViewed = async () => {
    try {
      const stored = await AsyncStorage.getItem('recentlyViewed');
      if (stored) {
        setRecentlyViewed(JSON.parse(stored));
      }
    } catch (error) {
      console.error('Error loading recently viewed:', error);
    }
  };

  const saveToRecentlyViewed = async (item: any) => {
    try {
      const updated = [item, ...recentlyViewed.filter(rv => rv.id !== item.id)].slice(0, 10);
      setRecentlyViewed(updated);
      await AsyncStorage.setItem('recentlyViewed', JSON.stringify(updated));
    } catch (error) {
      console.error('Error saving recently viewed:', error);
    }
  };

  const loadTrendingContent = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        console.log('No auth token, using mock data');
        setIsLoading(false);
        return;
      }

      // Load trending videos and popular streams in parallel
      const [trendingVideosResponse, popularStreamsResponse] = await Promise.all([
        fetch('http://localhost:8080/api/video/trending?limit=10', {
          headers: { 'Authorization': `Bearer ${token}` },
        }),
        fetch('http://localhost:8080/api/livestream/popular?limit=5', {
          headers: { 'Authorization': `Bearer ${token}` },
        }),
      ]);

      const trendingData = [];

      if (trendingVideosResponse.ok) {
        const videosData = await trendingVideosResponse.json();
        const formattedVideos = videosData.map((video: any) => ({
          id: video.ID,
          title: video.Title,
          views: video.ViewCount || 0,
          trending: true,
          duration: Math.floor(video.Metadata?.Duration || 0),
          type: 'video',
        }));
        trendingData.push(...formattedVideos);
      }

      if (popularStreamsResponse.ok) {
        const streamsData = await popularStreamsResponse.json();
        const formattedStreams = streamsData.map((stream: any) => ({
          id: stream.ID,
          title: stream.Title,
          views: stream.ViewerCount || 0,
          trending: true,
          duration: 'LIVE',
          type: 'stream',
        }));
        trendingData.push(...formattedStreams);
      }

      if (trendingData.length > 0) {
        setTrendingContent(trendingData);
      }
    } catch (error) {
      console.error('Error loading trending content:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleStartStream = () => {
    router.push('/screens/streamConfiguration');
  };

  const handleUploadVideo = () => {
    router.push('/screens/uploadVideo');
  };

  const handleQuickNavigation = (destination: string) => {
    switch (destination) {
      case 'Analytics':
        router.push('/analytics');
        break;
      case 'Settings':
        router.push('/settings');
        break;
      case 'Explore':
        router.push('/(tabs)/explore');
        break;
      case 'Profile':
        router.push('/(tabs)/profile');
        break;
      case 'Trending':
        // TODO: Create trending screen or filter explore screen
        Alert.alert('Trending', 'This feature will be available soon');
        break;
      default:
        Alert.alert('Navigation', `${destination} screen coming soon`);
    }
  };

  const getGreeting = () => {
    const hour = currentTime.getHours();
    if (hour < 12) return 'Good Morning';
    if (hour < 17) return 'Good Afternoon';
    return 'Good Evening';
  };

  const getTimeBasedEmoji = () => {
    const hour = currentTime.getHours();
    if (hour < 6) return '🌙';
    if (hour < 12) return '🌅';
    if (hour < 17) return '☀️';
    if (hour < 20) return '🌇';
    return '🌙';
  };

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <ScrollView style={styles.scrollView} showsVerticalScrollIndicator={false}>
        {/* Dynamic Welcome Header */}
        <View style={styles.welcomeSection}>
          <Text style={styles.greeting}>
            {getTimeBasedEmoji()} {getGreeting()}!
          </Text>
          <Text style={styles.subtitle}>Ready to create and discover amazing content?</Text>
          <Text style={styles.timeText}>
            {currentTime.toLocaleDateString('en-US', { 
              weekday: 'long', 
              month: 'short', 
              day: 'numeric' 
            })}
          </Text>
        </View>

        {/* Quick Action Buttons */}
        <View style={styles.quickActionsSection}>
          <Text style={styles.sectionTitle}>🚀 Quick Actions</Text>
          <View style={styles.quickActions}>
            <TouchableOpacity style={[styles.actionButton, styles.liveButton]} onPress={handleStartStream}>
              <Text style={styles.actionIcon}>🔴</Text>
              <Text style={styles.actionText}>Go Live</Text>
              <Text style={styles.actionSubtext}>Start streaming</Text>
            </TouchableOpacity>
            
            <TouchableOpacity style={[styles.actionButton, styles.uploadButton]} onPress={handleUploadVideo}>
              <Text style={styles.actionIcon}>📹</Text>
              <Text style={styles.actionText}>Upload Video</Text>
              <Text style={styles.actionSubtext}>Share content</Text>
            </TouchableOpacity>
          </View>
        </View>

        {/* Quick Navigation Shortcuts */}
        <View style={styles.shortcutsSection}>
          <Text style={styles.sectionTitle}>⚡ Quick Navigation</Text>
          <View style={styles.shortcuts}>
            <TouchableOpacity 
              style={styles.shortcutButton} 
              onPress={() => router.replace('/explore')}
            >
              <Text style={styles.shortcutIcon}>🔍</Text>
              <Text style={styles.shortcutText}>Explore</Text>
            </TouchableOpacity>
            
            <TouchableOpacity 
              style={styles.shortcutButton} 
              onPress={() => router.replace('/profile')}
            >
              <Text style={styles.shortcutIcon}>👤</Text>
              <Text style={styles.shortcutText}>Profile</Text>
            </TouchableOpacity>
            
            <TouchableOpacity 
              style={styles.shortcutButton} 
              onPress={() => router.replace('/settings')}
            >
              <Text style={styles.shortcutIcon}>⚙️</Text>
              <Text style={styles.shortcutText}>Settings</Text>
            </TouchableOpacity>
            
            <TouchableOpacity 
              style={styles.shortcutButton} 
              onPress={() => router.replace('/analytics')}
            >
              <Text style={styles.shortcutIcon}>📊</Text>
              <Text style={styles.shortcutText}>Analytics</Text>
            </TouchableOpacity>
          </View>
        </View>

        {/* Recently Viewed Section (Frontend-only with AsyncStorage) */}
        {recentlyViewed.length > 0 && (
          <View style={styles.section}>
            <View style={styles.sectionHeader}>
              <Text style={styles.sectionTitle}>📖 Recently Viewed</Text>
              <TouchableOpacity onPress={() => setRecentlyViewed([])}>
                <Text style={styles.clearText}>Clear</Text>
              </TouchableOpacity>
            </View>
            
            <FlatList
              data={recentlyViewed}
              renderItem={({ item }) => <RecentlyViewedCard item={item} />}
              keyExtractor={(item) => item.id}
              horizontal
              showsHorizontalScrollIndicator={false}
              contentContainerStyle={styles.horizontalList}
            />
          </View>
        )}

        {/* Trending Content Section */}
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>🔥 Trending Now</Text>
            <TouchableOpacity onPress={() => handleQuickNavigation('Trending')}>
              <Text style={styles.seeAllText}>See All</Text>
            </TouchableOpacity>
          </View>
          
          <View style={styles.trendingGrid}>
            {isLoading ? (
              <ActivityIndicator size="large" color="#007AFF" />
            ) : trendingContent.length > 0 ? (
              trendingContent.map((item) => (
                <View key={item.id} style={styles.trendingCardWrapper}>
                  <TrendingCard item={item} />
                </View>
              ))
            ) : (
              <Text style={styles.noContentText}>No trending content available</Text>
            )}
          </View>
        </View>

        {/* Bottom Padding for scroll */}
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
  welcomeSection: {
    backgroundColor: '#fff',
    paddingHorizontal: 20,
    paddingVertical: 24,
    borderBottomWidth: 1,
    borderBottomColor: '#e1e5e9',
  },
  greeting: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  subtitle: {
    fontSize: 16,
    color: '#666',
    marginBottom: 8,
  },
  timeText: {
    fontSize: 14,
    color: '#888',
  },
  quickActionsSection: {
    paddingHorizontal: 20,
    paddingVertical: 20,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 16,
  },
  quickActions: {
    flexDirection: 'row',
    gap: 12,
  },
  actionButton: {
    flex: 1,
    borderRadius: 16,
    paddingVertical: 24,
    paddingHorizontal: 16,
    alignItems: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.1,
    shadowRadius: 8,
    elevation: 4,
  },
  liveButton: {
    backgroundColor: '#ff3b30',
  },
  uploadButton: {
    backgroundColor: '#007AFF',
  },
  actionIcon: {
    fontSize: 32,
    marginBottom: 8,
  },
  actionText: {
    fontSize: 16,
    fontWeight: '600',
    color: '#fff',
    marginBottom: 4,
  },
  actionSubtext: {
    fontSize: 12,
    color: 'rgba(255,255,255,0.8)',
  },
  shortcutsSection: {
    paddingHorizontal: 20,
    paddingBottom: 20,
  },
  shortcuts: {
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  shortcutButton: {
    backgroundColor: '#fff',
    borderRadius: 12,
    paddingVertical: 16,
    paddingHorizontal: 12,
    alignItems: 'center',
    width: (width - 80) / 4, // 4 buttons with spacing
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  shortcutIcon: {
    fontSize: 24,
    marginBottom: 8,
  },
  shortcutText: {
    fontSize: 12,
    fontWeight: '500',
    color: '#1a1a1a',
  },
  section: {
    marginBottom: 24,
  },
  sectionHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 20,
    marginBottom: 16,
  },
  clearText: {
    fontSize: 14,
    color: '#ff3b30',
    fontWeight: '500',
  },
  seeAllText: {
    fontSize: 14,
    color: '#007AFF',
    fontWeight: '500',
  },
  horizontalList: {
    paddingLeft: 20,
  },
  recentCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    marginRight: 16,
    width: width * 0.6,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  recentCardThumbnail: {
    width: '100%',
    height: width * 0.3,
    backgroundColor: '#e1e5e9',
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    position: 'relative',
  },
  placeholderIcon: {
    fontSize: 32,
    opacity: 0.5,
  },
  liveIndicator: {
    position: 'absolute',
    top: 8,
    left: 8,
    backgroundColor: '#ff3b30',
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 4,
  },
  liveText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: '600',
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
  recentCardInfo: {
    padding: 12,
  },
  recentCardTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  recentCardMeta: {
    fontSize: 12,
    color: '#666',
  },
  trendingGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    paddingHorizontal: 10,
  },
  trendingCardWrapper: {
    width: '50%',
    paddingHorizontal: 10,
  },
  trendingCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    marginBottom: 16,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  trendingCardThumbnail: {
    width: '100%',
    height: width * 0.25,
    backgroundColor: '#e1e5e9',
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    position: 'relative',
  },
  trendingCardInfo: {
    padding: 12,
  },
  trendingCardTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  trendingCardMeta: {
    fontSize: 12,
    color: '#666',
  },
  bottomPadding: {
    height: 20,
  },
  noContentText: {
    textAlign: 'center',
    color: '#666',
    fontSize: 16,
    marginTop: 20,
  },
});
