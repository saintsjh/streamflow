import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity, StatusBar } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router } from 'expo-router';

type BackHeaderProps = {
  title?: string;
  subtitle?: string;
  onBackPress?: () => void;
  rightElement?: React.ReactNode;
  backgroundColor?: string;
  textColor?: string;
  showBorder?: boolean;
  backButtonStyle?: 'default' | 'close' | 'arrow';
  showBackButton?: boolean;
};

export default function BackHeader({
  title,
  subtitle,
  onBackPress,
  rightElement,
  backgroundColor = '#fff',
  textColor = '#1a1a1a',
  showBorder = true,
  backButtonStyle = 'default',
  showBackButton = true,
}: BackHeaderProps) {
  const handleBackPress = () => {
    if (onBackPress) {
      onBackPress();
    } else {
      // Check if we can go back, otherwise navigate to home
      try {
        if (router.canGoBack()) {
          router.back();
        } else {
          // Fallback to home tab if no navigation history
          router.replace('/(tabs)');
        }
      } catch (error) {
        // Fallback navigation in case of any routing errors
        router.replace('/(tabs)');
      }
    }
  };

  const getBackIcon = () => {
    switch (backButtonStyle) {
      case 'close':
        return '✕';
      case 'arrow':
        return '←';
      default:
        return '←';
    }
  };

  return (
    <SafeAreaView style={[styles.container, { backgroundColor }]} edges={['top']}>
      <StatusBar barStyle="dark-content" backgroundColor={backgroundColor} />
      <View style={[styles.header, showBorder && styles.headerBorder]}>
        <View style={styles.leftSection}>
          {showBackButton && (
            <TouchableOpacity 
              style={[
                styles.backButton, 
                backButtonStyle === 'close' && styles.closeButton
              ]} 
              onPress={handleBackPress}
            >
              <Text style={[styles.backIcon, { color: textColor }]}>
                {getBackIcon()}
              </Text>
            </TouchableOpacity>
          )}
          
          {title && (
            <View style={[styles.titleSection, !showBackButton && styles.titleSectionNoBack]}>
              <Text style={[styles.title, { color: textColor }]} numberOfLines={1}>
                {title}
              </Text>
              {subtitle && (
                <Text style={[styles.subtitle, { color: textColor + '80' }]} numberOfLines={1}>
                  {subtitle}
                </Text>
              )}
            </View>
          )}
        </View>
        
        {rightElement && (
          <View style={styles.rightSection}>
            {rightElement}
          </View>
        )}
      </View>
    </SafeAreaView>
  );
}

// Convenience component for modals/overlays
export function ModalHeader({ 
  title, 
  onClose, 
  rightElement 
}: { 
  title: string; 
  onClose: () => void; 
  rightElement?: React.ReactNode; 
}) {
  return (
    <BackHeader
      title={title}
      onBackPress={onClose}
      rightElement={rightElement}
      backButtonStyle="close"
      showBorder={true}
    />
  );
}

// Convenience component for simple screens
export function SimpleHeader({ 
  title, 
  subtitle,
  rightElement 
}: { 
  title: string; 
  subtitle?: string;
  rightElement?: React.ReactNode; 
}) {
  return (
    <BackHeader
      title={title}
      subtitle={subtitle}
      rightElement={rightElement}
      showBorder={true}
    />
  );
}

const styles = StyleSheet.create({
  container: {
    zIndex: 1000,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingVertical: 12,
    minHeight: 56,
  },
  headerBorder: {
    borderBottomWidth: 1,
    borderBottomColor: '#e1e5e9',
  },
  leftSection: {
    flexDirection: 'row',
    alignItems: 'center',
    flex: 1,
  },
  backButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 8,
    backgroundColor: 'rgba(0,0,0,0.05)',
  },
  closeButton: {
    backgroundColor: 'rgba(255, 59, 48, 0.1)',
  },
  backIcon: {
    fontSize: 20,
    fontWeight: '600',
  },
  titleSection: {
    flex: 1,
    marginLeft: 8,
  },
  titleSectionNoBack: {
    marginLeft: 0,
  },
  title: {
    fontSize: 20,
    fontWeight: '600',
    marginBottom: 2,
  },
  subtitle: {
    fontSize: 14,
  },
  rightSection: {
    marginLeft: 16,
  },
}); 