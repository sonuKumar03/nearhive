import { useState, useEffect } from 'react';
import { fetchApi } from '@/lib/api-client';
import { AuthResponse, User } from '@/types';

export function useAuth() {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    const savedToken = localStorage.getItem('nearhive_token');
    const savedUser = localStorage.getItem('nearhive_user');
    if (savedToken) {
      setToken(savedToken);
      if (savedUser) {
        try { setUser(JSON.parse(savedUser)); } catch {}
      }
    } else {
      autoLoginGuest();
    }
  }, []);

  async function autoLoginGuest() {
    try {
      const email = 'guest@nearhive.com';
      const password = 'guestpassword123';
      let res: AuthResponse;
      try {
        res = await fetchApi<AuthResponse>('/api/v1/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
      } catch {
        res = await fetchApi<AuthResponse>('/api/v1/auth/register', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
      }
      setToken(res.token);
      setUser(res.user);
      localStorage.setItem('nearhive_token', res.token);
      localStorage.setItem('nearhive_user', JSON.stringify(res.user));
    } catch (e) {
      console.warn('Guest login bypass:', e);
    }
  }

  function logout() {
    localStorage.removeItem('nearhive_token');
    localStorage.removeItem('nearhive_user');
    setToken(null);
    setUser(null);
  }

  return { token, user, logout };
}
