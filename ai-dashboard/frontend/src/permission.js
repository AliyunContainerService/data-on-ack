import router from './router'
import store from './store'
import NProgress from 'nprogress' // progress bar
import 'nprogress/nprogress.css' // progress bar style
import { getToken } from '@/utils/auth' // get token from cookie
import getPageTitle from '@/utils/get-page-title'
import { getUserInfo } from '@/api/user'
import { isEmpty } from '@/utils'

NProgress.configure({ showSpinner: false }) // NProgress Configuration

const whiteList = ['/login'] // no redirect whitelist

function handleLogin() {
  getUserInfo().then((response) => {
    if (response === undefined || response.data === undefined || response.data.user === undefined) {
      throw new Error('not login')
    }
    const user = response.data.user.spec
    const clusterInfo = { k8sVersion: response.data.k8sVersion }
    if (user.apiRoles === undefined) {
      throw new Error('no roles')
    }
    const roles = user.apiRoles
    if (roles.length < 1) {
      throw new Error('auth roles empty')
    }
    this.$store.dispatch('user/updateUser', { roles: roles, userName: user.userName, token: response.data.token, clusterInfo: clusterInfo }).then(() => {
      this.$store.dispatch('permission/generateRoutes', [roles, clusterInfo]).then((accessRoutes) => {
        // dynamically add accessible routes
        this.$router.addRoutes(accessRoutes)
      }).catch(error => {
        throw new Error(error)
      })

      // hack method to ensure that addRoutes is complete
      // set the replace: true, so the navigation will not leave a history record
      this.$router.push({ path: this.redirect || '/' })
    }).catch(error => {
      throw new Error(error)
    })
  }).catch(() => {
    /*
    this.$notify({
      title: '错误',
      message: '获取用户信息失败，请重新登陆',
      type: 'info',
      duration: 3000
    })
     */
    window.location = '/login/aliyun'
  })
}

router.beforeEach(async(to, from, next) => {
  // start progress bar
  NProgress.start()

  // set page title
  document.title = getPageTitle(to.meta.title)
  // determine whether the user has logged in
  var hasToken = !isEmpty(getToken())
  console.log('token:', hasToken)
  // const hasToken = true
  if (hasToken) {
    if (to.path === '/login/') {
      // if is logged in, redirect to the home page
      next({ path: '/' })
    } else {
      // determine whether the user has obtained his permission roles through getInfo
      const hasRoles = store.getters.roles && store.getters.roles.length > 0
      // const hasRoles = true
      console.log('roles:', hasRoles)
      if (hasRoles) {
        next()
      } else {
        try {
          // get user info
          // note: roles must be a object array! such as: ['admin'] or ,['developer','editor']
          const { roles, clusterInfo } = await store.dispatch('user/syncLoginInfo')
          // generate accessible routes map based on roles
          const accessRoutes = await store.dispatch('permission/generateRoutes', [roles, clusterInfo])

          // dynamically add accessible routes
          router.addRoutes(accessRoutes)

          // hack method to ensure that addRoutes is complete
          // set the replace: true, so the navigation will not leave a history record
          next({ ...to, replace: true })
        } catch (error) {
          // remove token and go to login page to re-login
          await store.dispatch('user/resetToken')
          // router.replace({ path: '/login' })
          handleLogin()
        }
      }
    }
  } else {
    /* has no token*/
    if (whiteList.indexOf(to.path) !== -1) {
      // in the free login whitelist, go directly
      next()
    } else {
      const hasToken = !isEmpty(getToken())
      if (hasToken) {
        next()
      } else {
        try {
          const { roles, clusterInfo } = await store.dispatch('user/syncLoginInfo')
          // generate accessible routes map based on roles
          const accessRoutes = await store.dispatch('permission/generateRoutes', [roles, clusterInfo])

          // dynamically add accessible routes
          router.addRoutes(accessRoutes)
          next({ ...to, replace: true })
        } catch (error) {
          // router.replace({ path: '/login' })
          handleLogin()
        }
      }
    }
  }
})

router.afterEach(() => {
  // finish progress bar
  NProgress.done()
})
