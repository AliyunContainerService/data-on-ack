import { create } from 'zustand'
import { get, post } from '@/api/client'

interface UserInfo {
  userName: string
  aliuid: string
  apiRoles: string[]
}

interface UserState {
  user: UserInfo | null
  k8sVersion: string
  loading: boolean
  fetchUserInfo: () => Promise<boolean>
  logout: () => Promise<void>
}

export const useUserStore = create<UserState>((set) => ({
  user: null,
  k8sVersion: '',
  loading: false,

  fetchUserInfo: async () => {
    set({ loading: true })
    try {
      const data: any = await get('/user/info')
      const user = data?.user
      const info: UserInfo | null = user
        ? {
            userName: user.spec?.userName || '',
            aliuid: user.spec?.aliuid || '',
            apiRoles: user.spec?.apiRoles || [],
          }
        : null
      set({ user: info, k8sVersion: data?.k8sVersion || '', loading: false })
      return info !== null
    } catch {
      set({ loading: false })
      return false
    }
  },

  logout: async () => {
    try {
      await post('/logout')
    } catch {
      // ignore
    }
    set({ user: null, k8sVersion: '' })
    window.location.href = '/login'
  },
}))
