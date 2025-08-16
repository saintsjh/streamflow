import React, { useState } from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet, Alert } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useAuth } from '@/contexts/AuthContext';
import { router } from 'expo-router';
import axios from 'axios';
  import { API_BASE_URL } from '@/config/api';

const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const { login } = useAuth();

  const handleLogin = async () => {
    console.log('[LOGIN] Initiating login flow', { email: email ? '***@' + email.split('@')[1] : 'empty' });
    
    if (!email || !password) {
      console.log('[LOGIN] Validation failed - missing fields', { 
        hasEmail: !!email, 
        hasPassword: !!password 
      });
      Alert.alert('Error', 'Please fill in all fields');
      return;
    }

    console.log('[LOGIN] Starting authentication request');
    setIsLoading(true);
    
    const startTime = Date.now();
    
    try {
      console.log('[LOGIN] Making API request', { baseUrl: API_BASE_URL });
      
      const response = await axios.post(`${API_BASE_URL}/user/login`, {
        email,
        password,
      });

      const requestDuration = Date.now() - startTime;
      console.log('[LOGIN] API response received', { 
        status: response.status, 
        hasToken: !!response.data.token,
        duration: `${requestDuration}ms`
      });

      if (response.data.token) {
        console.log('[LOGIN] Token received, calling auth context login');
        await login(response.data.token);
        console.log('[LOGIN] Login successful, navigating to tabs');
        router.replace('/(tabs)');
      } else {
        console.error('[LOGIN] No token in response', { responseData: response.data });
        throw new Error('Login failed');
      }
    } catch (error: any) {
      const requestDuration = Date.now() - startTime;
      
      if (error.response) {
        console.error('[LOGIN] HTTP error response', {
          status: error.response.status,
          statusText: error.response.statusText,
          data: error.response.data,
          duration: `${requestDuration}ms`
        });
      } else if (error.request) {
        console.error('[LOGIN] Network error - no response received', {
          message: error.message,
          duration: `${requestDuration}ms`
        });
      } else {
        console.error('[LOGIN] Request setup error', {
          message: error.message,
          duration: `${requestDuration}ms`
        });
      }
      
      const errorMessage = error.response?.data?.error || 'Invalid email or password';
      console.log('[LOGIN] Showing error alert to user', { errorMessage });
      Alert.alert('Login Failed', errorMessage);
    } finally {
      setIsLoading(false);
      console.log('[LOGIN] Login flow completed');
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.content}>
        <Text style={styles.title}>Welcome Back</Text>
        <Text style={styles.subtitle}>Sign in to your account</Text>
        
        <View style={styles.form}>
          <TextInput
            style={styles.input}
            placeholder="Email"
            placeholderTextColor="#999"
            value={email}
            onChangeText={setEmail}
            keyboardType="email-address"
            autoCapitalize="none"
            autoCorrect={false}
          />
          
          <TextInput
            style={styles.input}
            placeholder="Password"
            placeholderTextColor="#999"
            value={password}
            onChangeText={setPassword}
            secureTextEntry
            autoCapitalize="none"
            autoCorrect={false}
          />
          
          <TouchableOpacity 
            style={[styles.button, isLoading && styles.buttonDisabled]}
            onPress={handleLogin}
            disabled={isLoading}
          >
            <Text style={styles.buttonText}>
              {isLoading ? 'Signing In...' : 'Sign In'}
            </Text>
          </TouchableOpacity>
          <Text style={styles.signupText}>Don&apos;t have an account? </Text>
          <TouchableOpacity 
            style={[styles.signupButton, isLoading && styles.buttonDisabled]}
            onPress={() => router.replace('/(auth)/signup')}
            disabled={isLoading}
          >
            <Text style={styles.buttonText}>
              Sign Up
            </Text>
          </TouchableOpacity>
        </View>
      </View>
    </SafeAreaView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  content: {
    flex: 1,
    justifyContent: 'center',
    paddingHorizontal: 24,
  },
  title: {
    fontSize: 32,
    fontWeight: 'bold',
    textAlign: 'center',
    marginBottom: 8,
    color: '#1a1a1a',
  },
  subtitle: {
    fontSize: 16,
    textAlign: 'center',
    marginBottom: 48,
    color: '#666',
  },
  form: {
    gap: 16,
  },
  input: {
    height: 56,
    borderWidth: 1,
    borderColor: '#e1e1e1',
    borderRadius: 12,
    paddingHorizontal: 16,
    fontSize: 16,
    backgroundColor: '#f9f9f9',
  },
  button: {
    height: 56,
    backgroundColor: '#007AFF',
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    marginTop: 16,
  },
  buttonDisabled: {
    opacity: 0.6,
  },
  buttonText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
  },
  signupText: {
    textAlign: 'center',
    marginTop: 16,
    color: '#666',
  },
  signupButton: {
    height: 56,
    backgroundColor: '#007AFF',
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    marginTop: 16,
  },
});

export default Login; 