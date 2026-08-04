import { create } from 'zustand';
import i18n from '../i18n';

export interface UserInfo {
  aid: string;
  uid: string;
  name: string;
  loginName: string;
  role: string;
  namespaces: string[];
}

interface UserState {
  user: UserInfo | null;
  locale: string;
  loading: boolean;
  fetchUser: () => Promise<void>;
  logout: () => void;
  setLocale: (locale: string) => void;
}

export const useUserStore = create<UserState>((set) => ({
  user: null,
  locale: localStorage.getItem('ai-dev-console-lang') || 'zh',
  loading: false,

  fetchUser: async () => {
    set({ loading: true });
    try {
      const resp = await fetch('/api/v1/user/info', { credentials: 'include' });
      if (!resp.ok) throw new Error('unauthorized');
      const json = await resp.json();
      if (json.code === 10000) {
        set({ user: json.data, loading: false });
      } else {
        set({ user: null, loading: false });
      }
    } catch {
      set({ user: null, loading: false });
    }
  },

  logout: () => {
    set({ user: null });
  },

  setLocale: (locale: string) => {
    localStorage.setItem('ai-dev-console-lang', locale);
    i18n.changeLanguage(locale);
    set({ locale });
  },
}));
