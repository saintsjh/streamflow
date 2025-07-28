import React, { createContext, useContext, useState, useEffect } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { router } from 'expo-router';

type AuthContextType = {
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (token: string) => Promise<void>;
  logout: () => Promise<void>;
  signup: (token: string) => Promise<void>;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    checkAuthStatus();
  }, []);

  const checkAuthStatus = async () => {
    try {
      console.log('AuthContext: Checking authentication status...');
      const token = await AsyncStorage.getItem('userToken');
      const isAuth = !!token;
      console.log('AuthContext: Token found:', !!token);
      setIsAuthenticated(isAuth);
    } catch (error) {
      console.error('Error checking auth status:', error);
      setIsAuthenticated(false);
    } finally {
      setIsLoading(false);
      console.log('AuthContext: Finished checking auth status');
    }
  };

  const login = async (token: string) => {
    try {
      console.log('AuthContext: Logging in user...');
      await AsyncStorage.setItem('userToken', token);
      setIsAuthenticated(true);
      console.log('AuthContext: User logged in successfully');
    } catch (error) {
      console.error('Error storing token:', error);
      throw error;
    }
  };

  const logout = async () => {
    try {
      console.log('AuthContext: Logging out user...');
      await AsyncStorage.removeItem('userToken');
      setIsAuthenticated(false);
      console.log('AuthContext: User logged out successfully');
    } catch (error) {
      console.error('Error removing token:', error);
    }
  };

  const signup = async (token: string) => {
    try {
      console.log('AuthContext: Signing up user...');
      await AsyncStorage.setItem('userToken', token);
      setIsAuthenticated(true);
      console.log('AuthContext: User signed up successfully');
      router.replace('/(tabs)');
    } catch (error) {
      console.error('Error storing token:', error);
      throw error;
    }
  }

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout, signup }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
} 