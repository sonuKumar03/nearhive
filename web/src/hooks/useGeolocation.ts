import { useState, useCallback } from 'react';

export interface Coordinates {
  lat: number;
  lng: number;
}

export function useGeolocation() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const getCurrentLocation = useCallback((): Promise<Coordinates> => {
    return new Promise((resolve, reject) => {
      if (typeof window === 'undefined' || !navigator.geolocation) {
        const msg = 'Geolocation is not supported by your browser';
        setError(msg);
        reject(new Error(msg));
        return;
      }

      setLoading(true);
      setError(null);

      navigator.geolocation.getCurrentPosition(
        (pos) => {
          setLoading(false);
          resolve({
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
          });
        },
        (err) => {
          setLoading(false);
          let message = 'Failed to retrieve your location';
          if (err.code === err.PERMISSION_DENIED) {
            message = 'Location permission denied. Please allow access in browser settings.';
          } else if (err.code === err.POSITION_UNAVAILABLE) {
            message = 'Location position unavailable.';
          } else if (err.code === err.TIMEOUT) {
            message = 'Location request timed out.';
          }
          setError(message);
          reject(new Error(message));
        },
        { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 }
      );
    });
  }, []);

  return { getCurrentLocation, loading, error };
}
