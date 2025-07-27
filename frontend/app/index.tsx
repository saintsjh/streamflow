import { Redirect } from 'expo-router';
import { useAuth } from '@/contexts/AuthContext';
import { ActivityIndicator, View, Text } from 'react-native';
import { useEffect } from 'react';

export default function RootIndex() {
  const { isAuthenticated, isLoading } = useAuth();

  // Debug log to see authentication state
  useEffect(() => {
    console.log('RootIndex - isLoading:', isLoading, 'isAuthenticated:', isAuthenticated);
  }, [isLoading, isAuthenticated]);

  if (isLoading) {
    return (
      <View style={{ 
        flex: 1, 
        justifyContent: 'center', 
        alignItems: 'center', 
        backgroundColor: '#f8f9fa' 
      }}>
        <ActivityIndicator size="large" color="#007AFF" />
        <Text style={{ 
          marginTop: 16, 
          fontSize: 16, 
          color: '#666',
          textAlign: 'center'
        }}>
          Loading...
        </Text>
      </View>
    );
  }

  // Redirect based on authentication status
  if (isAuthenticated) {
    console.log('Redirecting to main app (tabs)');
    return <Redirect href="/(tabs)" />;
  } else {
    console.log('Redirecting to auth (login)');
    return <Redirect href="/(auth)/login" />;
  }
} 