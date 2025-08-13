import React, { useState, useMemo, useEffect } from 'react';
import { View, Text, StyleSheet, ScrollView, FlatList, TouchableOpacity, Dimensions, TextInput, ActivityIndicator, Alert } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useAuth } from '@/contexts/AuthContext';
import { API_BASE_URL } from '@/config/api';

const { width } = Dimensions.get('window');

// Mock data - replace with actual API calls later
const mockLiveStreams = [
  {
    id: '1',
    title: 'Gaming Session - Apex Legends',
    thumbnail: null, // Will be image URL
    createdAt: '2024-01-15T10:30:00Z',
    viewerCount: 145,
  },
  {
    id: '2',
    title: 'Music Production Live',
    thumbnail: null,
    createdAt: '2024-01-15T09:15:00Z',
    viewerCount: 89,
  },
  {
    id: '3',
    title: 'Cooking Show - Italian Pasta',
    thumbnail: null,
    createdAt: '2024-01-15T11:45:00Z',
    viewerCount: 234,
  },
];

const mockVideos = [
  {
    id: '1',
    title: 'React Native Tutorial - Getting Started',
    thumbnail: null,
    createdAt: '2024-01-14T14:20:00Z',
    duration: '15:32',
  },
  {
    id: '2',
    title: 'Beautiful Sunset Timelapse',
    thumbnail: null,
    createdAt: '2024-01-14T16:45:00Z',
    duration: '3:45',
  },
  {
    id: '3',
    title: 'JavaScript Advanced Concepts',
    thumbnail: null,
    createdAt: '2024-01-13T12:30:00Z',
    duration: '28:15',
  },
  {
    id: '4',
    title: 'Mountain Hiking Adventure',
    thumbnail: null,
    createdAt: '2024-01-13T08:20:00Z',
    duration: '12:08',
  },
];

const formatTimeAgo = (dateString: string) => {
  const now = new Date();
  const createdAt = new Date(dateString);
  const diffInHours = Math.floor((now.getTime() - createdAt.getTime()) / (1000 * 60 * 60));
  
  if (diffInHours < 1) return 'Just now';
  if (diffInHours < 24) return `${diffInHours}h ago`;
  return `${Math.floor(diffInHours / 24)}d ago`;
};

const LiveStreamCard = ({ item }: { item: any }) => (
  <TouchableOpacity style={styles.card}>
    <View style={styles.thumbnail}>
      <View style={styles.liveIndicator}>
        <Text style={styles.liveText}>LIVE</Text>
      </View>
      <Text style={styles.placeholderText}>📹</Text>
    </View>
    <View style={styles.cardContent}>
      <Text style={styles.cardTitle} numberOfLines={2}>{item.title}</Text>
      <Text style={styles.cardMeta}>{item.viewerCount} viewers • {formatTimeAgo(item.createdAt)}</Text>
    </View>
  </TouchableOpacity>
);

const VideoCard = ({ item }: { item: any }) => (
  <TouchableOpacity style={styles.card}>
    <View style={styles.thumbnail}>
      <View style={styles.durationBadge}>
        <Text style={styles.durationText}>{item.duration}</Text>
      </View>
      <Text style={styles.placeholderText}>🎬</Text>
    </View>
    <View style={styles.cardContent}>
      <Text style={styles.cardTitle} numberOfLines={2}>{item.title}</Text>
      <Text style={styles.cardMeta}>{formatTimeAgo(item.createdAt)}</Text>
    </View>
  </TouchableOpacity>
);

export default function ExploreScreen() {
  const { logout } = useAuth();
  const [searchQuery, setSearchQuery] = useState('');
  const [liveStreams, setLiveStreams] = useState(mockLiveStreams);
  const [videos, setVideos] = useState(mockVideos);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      if (!token) {
        Alert.alert('Authentication Error', 'Please log in again.');
        await logout();
        return;
      }

      // Load livestreams and videos in parallel
      const [streamsResponse, videosResponse] = await Promise.all([
        fetch(`${API_BASE_URL}/api/livestream/streams`, {
          headers: { 'Authorization': `Bearer ${token}` },
        }),
        fetch(`${API_BASE_URL}/api/video/list?page=1&limit=20`, {
          headers: { 'Authorization': `Bearer ${token}` },
        }),
      ]);

      if (streamsResponse.ok) {
        const streamsData = await streamsResponse.json();
        // Convert backend data to match frontend format
        const formattedStreams = streamsData.map((stream: any) => ({
          id: stream.ID,
          title: stream.Title,
          thumbnail: null,
          createdAt: stream.CreatedAt,
          viewerCount: stream.ViewerCount || 0,
        }));
        setLiveStreams(formattedStreams);
      }

      if (videosResponse.ok) {
        const videosData = await videosResponse.json();
        // Convert backend data to match frontend format  
        const formattedVideos = videosData.map((video: any) => ({
          id: video.ID,
          title: video.Title,
          thumbnail: null,
          createdAt: video.CreatedAt,
          duration: Math.floor(video.Metadata?.Duration || 0),
        }));
        setVideos(formattedVideos);
      }
    } catch (error) {
      console.error('Error loading data:', error);
      Alert.alert('Error', 'Failed to load content. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  // Filter data based on search query
  const filteredLiveStreams = useMemo(() => {
    if (!searchQuery.trim()) return liveStreams;
    return liveStreams.filter(stream =>
      stream.title.toLowerCase().includes(searchQuery.toLowerCase())
    );
  }, [searchQuery, liveStreams]);

  const filteredVideos = useMemo(() => {
    if (!searchQuery.trim()) return videos;
    return videos.filter(video =>
      video.title.toLowerCase().includes(searchQuery.toLowerCase())
    );
  }, [searchQuery, videos]);

  const clearSearch = () => {
    setSearchQuery('');
  };

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <ScrollView style={styles.scrollView} showsVerticalScrollIndicator={false}>
        {/* Search Section */}
        <View style={styles.searchSection}>
          <View style={styles.searchContainer}>
            <Text style={styles.searchIcon}>🔍</Text>
            <TextInput
              style={styles.searchInput}
              placeholder="Search livestreams and videos..."
              placeholderTextColor="#999"
              value={searchQuery}
              onChangeText={setSearchQuery}
              autoCapitalize="none"
              autoCorrect={false}
            />
            {searchQuery.length > 0 && (
              <TouchableOpacity onPress={clearSearch} style={styles.clearButton}>
                <Text style={styles.clearButtonText}>✕</Text>
              </TouchableOpacity>
            )}
          </View>
        </View>

        {/* Search Results or Default Content */}
        {searchQuery.trim() ? (
          <>
            {/* Search Results */}
            {filteredLiveStreams.length > 0 && (
              <View style={styles.section}>
                <View style={styles.sectionHeader}>
                  <Text style={styles.sectionTitle}>🔴 Live Streams ({filteredLiveStreams.length})</Text>
                </View>
                
                <FlatList
                  data={filteredLiveStreams}
                  renderItem={({ item }) => <LiveStreamCard item={item} />}
                  keyExtractor={(item) => item.id}
                  horizontal
                  showsHorizontalScrollIndicator={false}
                  contentContainerStyle={styles.horizontalList}
                />
              </View>
            )}

            {filteredVideos.length > 0 && (
              <View style={styles.section}>
                <View style={styles.sectionHeader}>
                  <Text style={styles.sectionTitle}>📺 Videos ({filteredVideos.length})</Text>
                </View>
                
                <View style={styles.videoGrid}>
                  {filteredVideos.map((item) => (
                    <View key={item.id} style={styles.videoCardWrapper}>
                      <VideoCard item={item} />
                    </View>
                  ))}
                </View>
              </View>
            )}

            {filteredLiveStreams.length === 0 && filteredVideos.length === 0 && (
              <View style={styles.noResults}>
                <Text style={styles.noResultsText}>🤔 No results found</Text>
                <Text style={styles.noResultsSubtext}>
                  Try searching with different keywords
                </Text>
              </View>
            )}
          </>
        ) : (
          <>
                         {/* Default Content - Live Streams Section */}
             <View style={styles.section}>
               <View style={styles.sectionHeader}>
                 <Text style={styles.sectionTitle}>🔴 Live Streams</Text>
                 <TouchableOpacity>
                   <Text style={styles.seeAllText}>See All</Text>
                 </TouchableOpacity>
               </View>
               
               <FlatList
                 data={mockLiveStreams}
                 renderItem={({ item }) => <LiveStreamCard item={item} />}
                 keyExtractor={(item) => item.id}
                 horizontal
                 showsHorizontalScrollIndicator={false}
                 contentContainerStyle={styles.horizontalList}
               />
             </View>

             {/* Videos Section */}
             <View style={styles.section}>
               <View style={styles.sectionHeader}>
                 <Text style={styles.sectionTitle}>📺 Recent Videos</Text>
                 <TouchableOpacity>
                   <Text style={styles.seeAllText}>See All</Text>
                 </TouchableOpacity>
               </View>
               
               <View style={styles.videoGrid}>
                 {mockVideos.map((item) => (
                   <View key={item.id} style={styles.videoCardWrapper}>
                     <VideoCard item={item} />
                   </View>
                 ))}
               </View>
             </View>
           </>
         )}
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
  searchSection: {
    backgroundColor: '#fff',
    paddingHorizontal: 20,
    paddingVertical: 16,
    borderBottomWidth: 1,
    borderBottomColor: '#e1e5e9',
  },
  searchContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#f8f9fa',
    borderRadius: 12,
    paddingHorizontal: 16,
    height: 48,
  },
  searchIcon: {
    fontSize: 16,
    marginRight: 12,
    opacity: 0.6,
  },
  searchInput: {
    flex: 1,
    fontSize: 16,
    color: '#1a1a1a',
  },
  clearButton: {
    padding: 4,
    marginLeft: 8,
  },
  clearButtonText: {
    fontSize: 16,
    color: '#666',
    fontWeight: '600',
  },
  noResults: {
    alignItems: 'center',
    paddingVertical: 60,
    paddingHorizontal: 40,
  },
  noResultsText: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  noResultsSubtext: {
    fontSize: 16,
    color: '#666',
    textAlign: 'center',
  },
  section: {
    marginTop: 16,
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
  horizontalList: {
    paddingLeft: 20,
  },
  videoGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    paddingHorizontal: 10,
  },
  videoCardWrapper: {
    width: '50%',
    paddingHorizontal: 10,
  },
  card: {
    backgroundColor: '#fff',
    borderRadius: 12,
    marginRight: 16,
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
    width: width * 0.4,
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
  cardContent: {
    padding: 12,
  },
  cardTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 4,
    lineHeight: 18,
  },
  cardMeta: {
    fontSize: 12,
    color: '#666',
  },
});
