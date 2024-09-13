import { logout, getUserInfo } from '@/api/user'
import { setToken, setRoles, removeToken, setClusterInfo } from '@/utils/auth'
import { resetRouter } from '@/router'

const getDefaultState = () => {
  return {
    token: '',
    name: '',
    avatar: '',
    clusterInfo: {},
    roles: []
  }
}

const state = getDefaultState()

const mutations = {
  RESET_STATE: (state) => {
    Object.assign(state, getDefaultState())
  },
  SET_TOKEN: (state, token) => {
    state.token = token
    setToken(token)
  },
  SET_NAME: (state, name) => {
    state.name = name
  },
  SET_AVATAR: (state, avatar) => {
    state.avatar = avatar
  },
  SET_ROLES: (state, roles) => {
    state.roles = roles
    setRoles(roles)
  },
  SET_CLUSTER_INFO: (state, clusterInfo) => {
    state.clusterInfo = clusterInfo
    setClusterInfo(clusterInfo)
  }
}

const actions = {

  updateUser({ commit }, userInfo) {
    const { roles, userName, token, clusterInfo } = userInfo
    // roles must be a non-empty array
    if (!roles || roles.length <= 0) {
      this.this.$notify({
        title: '角色格式错误',
        message: '必须为数组',
        type: 'error'
      })
      return
    }
    commit('SET_TOKEN', token)
    commit('SET_ROLES', roles)
    commit('SET_NAME', userName)
    commit('SET_CLUSTER_INFO', clusterInfo)
    const avatar = 'https://oss.aliyuncs.com/aliyun_id_photo_bucket/default_handsome.jpg'
    commit('SET_AVATAR', avatar)
  },

  // get user info
  syncLoginInfo({ commit, state }) {
    return new Promise((resolve, reject) => {
      getUserInfo().then(response => {
        if (response === undefined || response.data === undefined || response.data.user === undefined) {
          reject('Verification failed, please Login again.')
          return
        }
        const userInfo = response.data.user.spec
        const clusterInfo = { k8sVersion: response.data.k8sVersion }
        if (userInfo.apiRoles === undefined) {
          return
        }
        const roles = userInfo.apiRoles
        const username = userInfo.userName
        // roles must be a non-empty array
        if (!roles || roles.length <= 0) {
          reject('getInfo: roles must be a non-null array!')
        }

        // const avatar = 'https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif'
        const avatar = 'https://oss.aliyuncs.com/aliyun_id_photo_bucket/default_handsome.jpg'
        commit('SET_ROLES', roles)
        commit('SET_NAME', username)
        commit('SET_AVATAR', avatar)
        commit('SET_TOKEN', response.data.token)
        commit('SET_CLUSTER_INFO', clusterInfo)
        const user = {
          name: username,
          roles: roles,
          avatar: avatar,
          clusterInfo: clusterInfo
        }
        resolve(user)
      }).catch(error => {
        console.log('get user info error:', error)
        reject(error)
      })
    })
  },

  // user logout
  logout({ commit }, token) {
    return new Promise((resolve, reject) => {
      logout(token).then(() => {
        removeToken() // must remove  token  first
        resetRouter()
        commit('RESET_STATE')
        resolve()
      }).catch(error => {
        reject(error)
      })
    })
  },

  // remove token
  resetToken({ commit }) {
    return new Promise(resolve => {
      removeToken() // must remove  token  first
      commit('RESET_STATE')
      resolve()
    })
  }
}

export default {
  namespaced: true,
  state,
  mutations,
  actions
}

