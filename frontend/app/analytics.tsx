import React, { useState, useEffect } from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity, Dimensions, Alert, ActivityIndicator } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useAuth } from '@/contexts/AuthContext';
import BackHeader from '@/components/BackHeader';
import axios from 'axios';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { API_BASE_URL } from '@/config/api';

// Simple error handler for API errors
const handleApiError = (error: any): string => {
  if (error?.response?.data?.message) {
    return error.response.data.message;
  }
  if (error?.message) {
    return error.message;
  }
  return 'An unexpected error occurred';
};
        
const { width } = Dimensions.get('window');

// Mock analytics data - replace with actual API calls
const mockAnalyticsData = {
  overview: {
    totalVideos: 12,
    totalStreams: 8,
    totalViews: 15420,
    totalWatchTime: 2340, // in minutes
    subscribers: 234,
    avgViewDuration: 4.2, // in minutes
  },
  recentPerformance: {
    last7Days: {
      views: 1240,
      watchTime: 340,
      newSubscribers: 18,
    },
    last30Days: {
      views: 4650,
      watchTime: 1200,
      newSubscribers: 67,
    },
  },
  topVideos: [
    {
      id: '1',
      title: 'React Native Tutorial - Advanced',
      views: 3240,
      watchTime: 280,
      engagement: 85,
      publishedDays: 12,
    },
    {
      id: '2',
      title: 'JavaScript Performance Tips',
      views: 2890,
      watchTime: 240,
      engagement: 78,
      publishedDays: 8,
    },
    {
      id: '3',
      title: 'Mobile App Design Principles',
      views: 1980,
      watchTime: 180,
      engagement: 92,
      publishedDays: 5,
    },
  ],
  streamStats: {
    totalStreams: 8,
    totalStreamTime: 680, // in minutes
    avgViewers: 45,
    peakViewers: 234,
    chatMessages: 1240,
  }
};

const formatNumber = (num: number) => {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
  return num.toString();
};

const formatDuration = (minutes: number) => {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  if (hours > 0) {
    return `${hours}h ${mins}m`;
  }
  return `${mins}m`;
};

const StatCard = ({ title, value, subtitle, trend, color = '#007AFF' }: any) => (
  <View style={[styles.statCard, { borderLeftColor: color }]}>
    <Text style={styles.statTitle}>{title}</Text>
    <Text style={styles.statValue}>{value}</Text>
    <Text style={styles.statSubtitle}>{subtitle}</Text>
    {trend && (
      <Text style={[styles.trend, { color: trend.positive ? '#34C759' : '#FF3B30' }]}>
        {trend.positive ? '↗' : '↘'} {trend.value}
      </Text>
    )}
  </View>
);

const VideoPerformanceCard = ({ video }: any) => (
  <View style={styles.videoCard}>
    <View style={styles.videoInfo}>
      <Text style={styles.videoTitle} numberOfLines={2}>{video.title}</Text>
      <Text style={styles.videoMeta}>{video.publishedDays} days ago</Text>
    </View>
    <View style={styles.videoStats}>
      <View style={styles.videoStatItem}>
        <Text style={styles.videoStatValue}>{formatNumber(video.views)}</Text>
        <Text style={styles.videoStatLabel}>Views</Text>
      </View>
      <View style={styles.videoStatItem}>
        <Text style={styles.videoStatValue}>{formatDuration(video.watchTime)}</Text>
        <Text style={styles.videoStatLabel}>Watch Time</Text>
      </View>
      <View style={styles.videoStatItem}>
        <Text style={styles.videoStatValue}>{video.engagement}%</Text>
        <Text style={styles.videoStatLabel}>Engagement</Text>
      </View>
    </View>
  </View>
);

export default function AnalyticsScreen() {
  const { logout } = useAuth();
  const [selectedPeriod, setSelectedPeriod] = useState('7days');
  const [analyticsData, setAnalyticsData] = useState(mockAnalyticsData);
  const [isLoading, setIsLoading] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    loadAnalyticsData();
  }, []);

  const loadAnalyticsData = async () => {
    setIsLoading(true);
    try {
      const token = await AsyncStorage.getItem('userToken');
      
      // Load analytics data from multiple endpoints
      const [overviewResponse, videosResponse, streamsResponse] = await Promise.allSettled([
        axios.get(`${API_BASE_URL}/api/analytics/overview`, {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/analytics/videos`, {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/analytics/streams`, {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
      ]);

      const newAnalyticsData = { ...mockAnalyticsData };

      // Update with real data if available
      if (overviewResponse.status === 'fulfilled' && overviewResponse.value.data) {
        newAnalyticsData.overview = {
          ...newAnalyticsData.overview,
          ...overviewResponse.value.data,
        };
      }

      if (videosResponse.status === 'fulfilled' && videosResponse.value.data) {
        newAnalyticsData.topVideos = videosResponse.value.data.topVideos || newAnalyticsData.topVideos;
        newAnalyticsData.recentPerformance = videosResponse.value.data.recentPerformance || newAnalyticsData.recentPerformance;
      }

      if (streamsResponse.status === 'fulfilled' && streamsResponse.value.data) {
        newAnalyticsData.streamStats = {
          ...newAnalyticsData.streamStats,
          ...streamsResponse.value.data,
        };
      }

      setAnalyticsData(newAnalyticsData);
    } catch (error: any) {
      console.error('Error loading analytics data:', error);
      const errorMessage = error.response?.data?.error || 'Failed to load analytics data';
      Alert.alert('Error', errorMessage);

    } finally {
      setIsLoading(false);
    }
  };

  const refreshAnalytics = async () => {
    setIsRefreshing(true);
    try {
      await loadAnalyticsData();
      Alert.alert('Success', 'Analytics data refreshed!');
    } catch (error) {
      console.error('Error refreshing analytics:', error);
    } finally {
      setIsRefreshing(false);
    }
  };

  const getPeriodData = () => {
    return selectedPeriod === '7days' 
      ? analyticsData.recentPerformance.last7Days
      : analyticsData.recentPerformance.last30Days;
  };

  const periodData = getPeriodData();

  return (
    <SafeAreaView style={styles.container} edges={['bottom']}>
      <BackHeader 
        title="Analytics" 
        subtitle="Track your content performance"
        rightElement={
          <TouchableOpacity 
            style={styles.refreshButton}
            onPress={refreshAnalytics}
            disabled={isRefreshing}
          >
            {isRefreshing ? (
              <ActivityIndicator size="small" color="#007AFF" />
            ) : (
              <Text style={styles.refreshIcon}>↻</Text>
            )}
          </TouchableOpacity>
        }
      />
      <ScrollView style={styles.scrollView} showsVerticalScrollIndicator={false}>
        {/* Period Selector */}
        <View style={styles.periodSelector}>
          <TouchableOpacity
            style={[styles.periodButton, selectedPeriod === '7days' && styles.periodButtonActive]}
            onPress={() => setSelectedPeriod('7days')}
          >
            <Text style={[styles.periodButtonText, selectedPeriod === '7days' && styles.periodButtonTextActive]}>
              Last 7 Days
            </Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={[styles.periodButton, selectedPeriod === '30days' && styles.periodButtonActive]}
            onPress={() => setSelectedPeriod('30days')}
          >
            <Text style={[styles.periodButtonText, selectedPeriod === '30days' && styles.periodButtonTextActive]}>
              Last 30 Days
            </Text>
          </TouchableOpacity>
        </View>

        {/* Overview Stats */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>📊 Overview</Text>
          <View style={styles.statsGrid}>
            <StatCard
              title="Total Views"
              value={formatNumber(analyticsData.overview.totalViews)}
              subtitle="All time"
              color="#007AFF"
            />
            <StatCard
              title="Watch Time"
              value={formatDuration(analyticsData.overview.totalWatchTime)}
              subtitle="Total hours watched"
              color="#34C759"
            />
            <StatCard
              title="Subscribers"
              value={formatNumber(analyticsData.overview.subscribers)}
              subtitle="Total subscribers"
              color="#FF9500"
            />
            <StatCard
              title="Content"
              value={analyticsData.overview.totalVideos + analyticsData.overview.totalStreams}
              subtitle={`${analyticsData.overview.totalVideos} videos, ${analyticsData.overview.totalStreams} streams`}
              color="#AF52DE"
            />
          </View>
        </View>

        {/* Recent Performance */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>📈 Recent Performance</Text>
          <View style={styles.recentStats}>
            <StatCard
              title="Views"
              value={formatNumber(periodData.views)}
              subtitle={selectedPeriod === '7days' ? 'Last 7 days' : 'Last 30 days'}
              trend={{ positive: true, value: '+12%' }}
              color="#007AFF"
            />
            <StatCard
              title="Watch Time"
              value={formatDuration(periodData.watchTime)}
              subtitle="Total watch time"
              trend={{ positive: true, value: '+8%' }}
              color="#34C759"
            />
            <StatCard
              title="New Subscribers"
              value={periodData.newSubscribers}
              subtitle="New followers"
              trend={{ positive: false, value: '-3%' }}
              color="#FF9500"
            />
          </View>
        </View>

        {/* Top Performing Videos */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>🏆 Top Performing Content</Text>
          {isLoading ? (
            <ActivityIndicator size="large" color="#007AFF" style={styles.loadingIndicator} />
          ) : (
            analyticsData.topVideos.map((video) => (
              <VideoPerformanceCard key={video.id} video={video} />
            ))
          )}
        </View>

        {/* Live Stream Analytics */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>🔴 Live Stream Analytics</Text>
          <View style={styles.streamStatsGrid}>
            <StatCard
              title="Total Streams"
              value={analyticsData.streamStats.totalStreams}
              subtitle="Streams completed"
              color="#FF3B30"
            />
            <StatCard
              title="Stream Time"
              value={formatDuration(analyticsData.streamStats.totalStreamTime)}
              subtitle="Total live time"
              color="#FF3B30"
            />
            <StatCard
              title="Avg Viewers"
              value={analyticsData.streamStats.avgViewers}
              subtitle={`Peak: ${analyticsData.streamStats.peakViewers}`}
              color="#FF3B30"
            />
            <StatCard
              title="Chat Messages"
              value={formatNumber(analyticsData.streamStats.chatMessages)}
              subtitle="Total engagement"
              color="#FF3B30"
            />
          </View>
        </View>

        {/* Insights & Recommendations */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>💡 Insights & Tips</Text>
          <View style={styles.insightsCard}>
            <Text style={styles.insightTitle}>🎯 Performance Insights</Text>
            <Text style={styles.insightText}>
              • Your videos perform best on weekends{'\n'}
              • Tutorial content has 40% higher engagement{'\n'}
              • Live streams peak at 8 PM - 10 PM{'\n'}
              • Short-form content (5-10 min) gets more views
            </Text>
          </View>
          
          <View style={styles.insightsCard}>
            <Text style={styles.insightTitle}>📈 Growth Recommendations</Text>
            <Text style={styles.insightText}>
              • Post consistently 2-3 times per week{'\n'}
              • Engage with viewers in first hour after upload{'\n'}
              • Create more tutorial content{'\n'}
              • Consider live streaming weekly
            </Text>
          </View>
        </View>

        {/* Bottom padding */}
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
  periodSelector: {
    flexDirection: 'row',
    backgroundColor: '#fff',
    marginHorizontal: 20,
    marginTop: 16,
    borderRadius: 12,
    padding: 4,
  },
  periodButton: {
    flex: 1,
    paddingVertical: 12,
    paddingHorizontal: 16,
    alignItems: 'center',
    borderRadius: 8,
  },
  periodButtonActive: {
    backgroundColor: '#007AFF',
  },
  periodButtonText: {
    fontSize: 14,
    fontWeight: '600',
    color: '#666',
  },
  periodButtonTextActive: {
    color: '#fff',
  },
  section: {
    marginTop: 24,
    paddingHorizontal: 20,
  },
  sectionTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 16,
  },
  statsGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    marginHorizontal: -8,
  },
  recentStats: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    marginHorizontal: -8,
  },
  streamStatsGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    marginHorizontal: -8,
  },
  statCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    marginHorizontal: 8,
    marginBottom: 16,
    width: (width - 56) / 2,
    borderLeftWidth: 4,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  statTitle: {
    fontSize: 14,
    color: '#666',
    marginBottom: 8,
  },
  statValue: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  statSubtitle: {
    fontSize: 12,
    color: '#888',
    marginBottom: 4,
  },
  trend: {
    fontSize: 12,
    fontWeight: '600',
  },
  videoCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  videoInfo: {
    marginBottom: 12,
  },
  videoTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 4,
  },
  videoMeta: {
    fontSize: 12,
    color: '#666',
  },
  videoStats: {
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  videoStatItem: {
    alignItems: 'center',
    flex: 1,
  },
  videoStatValue: {
    fontSize: 18,
    fontWeight: 'bold',
    color: '#1a1a1a',
    marginBottom: 2,
  },
  videoStatLabel: {
    fontSize: 12,
    color: '#666',
  },
  insightsCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  insightTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1a1a1a',
    marginBottom: 8,
  },
  insightText: {
    fontSize: 14,
    color: '#666',
    lineHeight: 20,
  },
  bottomPadding: {
    height: 20,
  },
  refreshButton: {
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  refreshIcon: {
    fontSize: 20,
    color: '#007AFF',
  },
  loadingIndicator: {
    marginVertical: 20,
  },
});
