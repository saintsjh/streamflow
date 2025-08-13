import React, { useState } from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet, Alert } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router } from 'expo-router';
import axios from "axios";
import { API_BASE_URL } from '@/config/api';

const SignUp = () => {
    const [userName, setUserName] = useState('');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [isLoading, setIsLoading] = useState(false);

    const handleSignUp = async () => {
        if (!userName || !email || !password) {
            Alert.alert('Error', 'Please fill in all fields');
            return;
        }

        setIsLoading(true);
        try {
            const response = await axios.post(`${API_BASE_URL}/user/register`, {
                user_name: userName,
                email,
                password,
            });

            if (response.status === 201) {
                Alert.alert('Success', 'Account created successfully!');
                router.replace('/(auth)/login');
            } else {
                console.log(response.data);
            }
        } catch (error: any) {
            const errorMessage = error.response?.data?.error || 'An unexpected error occurred.';
            Alert.alert('Sign Up Failed', errorMessage);
        } finally {
            setIsLoading(false);
        }
    };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.content}>
        <Text style={styles.title}>Create Account</Text>
        
        <View style={styles.form}>
        <TextInput
            style={styles.input}
            placeholder="Username"
            placeholderTextColor="#999"
            value={userName}
            onChangeText={setUserName}
            autoCapitalize="none"
          />
          <TextInput
            style={styles.input}
            placeholder="Email"
            placeholderTextColor="#999"
            value={email}
            onChangeText={setEmail}
            keyboardType="email-address"
            autoCapitalize="none"
          />
          <TextInput
            style={styles.input}
            placeholder="Password"
            placeholderTextColor="#999"
            value={password}
            onChangeText={setPassword}
            secureTextEntry
          />
          
          <TouchableOpacity 
            style={[styles.button, isLoading && styles.buttonDisabled]}
            onPress={handleSignUp}
            disabled={isLoading}
          >
            <Text style={styles.buttonText}>
              {isLoading ? 'Creating Account...' : 'Sign Up'}
            </Text>
          </TouchableOpacity>
        </View>
        
        <TouchableOpacity onPress={() => router.replace('/(auth)/login')} style={styles.loginLink}>
          <Text style={styles.loginText}>Already have an account? Sign In</Text>
        </TouchableOpacity>
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
      marginBottom: 48,
      color: '#1a1a1a',
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
    loginLink: {
      marginTop: 24,
      alignItems: 'center',
    },
    loginText: {
      color: '#007AFF',
      fontSize: 16,
    },
  });

export default SignUp;
