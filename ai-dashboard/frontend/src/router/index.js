import Vue from 'vue'
import Router from 'vue-router'

Vue.use(Router)

/* Layout */
import Layout from '@/layout'

/**
 * Note: sub-menu only appear when route children.length >= 1
 * Detail see: https://panjiachen.github.io/vue-element-admin-site/guide/essentials/router-and-nav.html
 *
 * hidden: true                   if set true, item will not show in the sidebar(default is false)
 * alwaysShow: true               if set true, will always show the root menu
 *                                if not set alwaysShow, when item has more than one children route,
 *                                it will becomes nested mode, otherwise not show the root menu
 * redirect: noRedirect           if set noRedirect will no redirect in the breadcrumb
 * name:'router-name'             the name is used by <keep-alive> (must set!!!)
 * meta : {
    roles: ['admin','editor']    control the page roles (you can set multiple roles)
    title: 'title'               the name show in sidebar and breadcrumb (recommend set)
    icon: 'svg-name'/'el-icon-x' the icon show in the sidebar
    breadcrumb: false            if set false, the item will hidden in breadcrumb(default is true)
    activeMenu: '/example/list'  if set path, the sidebar will highlight the path you set
  }
 */

/**
 * constantRoutes
 * a base page that does not have permission requirements
 * all roles can be accessed
 */
export const constantRoutes = [
  {
    path: '/login',
    component: () => import('@/views/login/index'),
    hidden: true
  },
  {
    path: '/404',
    component: () => import('@/views/404'),
    hidden: true
  },
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    children: [{
      path: 'dashboard',
      name: 'Dashboard',
      component: () => import('@/views/dashboard/index'),
      meta: { title: 'dashboard', icon: 'dashboard' }
    }]
  }
]

/**
 * asyncRoutes
 * the routes that need to be dynamically loaded based on user roles
 */
export const asyncRoutes = [
  // group
  {
    path: '/group',
    component: Layout,
    redirect: '/group/list',
    alwaysShow: true,
    name: 'Group',
    meta: {
      title: 'quota',
      icon: 'el-icon-menu',
      roles: ['admin']
    },
    children: [
      {
        path: 'list',
        component: () => import('@/views/group/ElasticQuotaTree'),
        name: 'GroupList',
        meta: { title: 'quotaList' }
      }
    ]
  },
  // researcher
  {
    path: '/researcher',
    component: Layout,
    redirect: '/researcher/list',
    alwaysShow: true,
    name: 'Researcher',
    meta: {
      title: 'user',
      icon: 'el-icon-user',
      roles: ['admin']
    },
    children: [
      {
        path: 'list',
        component: () => import('@/views/researcher/ResearcherList'),
        name: 'ResearcherList',
        meta: { title: 'userList', roles: ['admin'] }
      },
      {
        path: 'group',
        component: () => import('@/views/researcher/ResearcherGroup'),
        name: 'ResearcherGroup',
        meta: { title: 'userGroup', roles: ['admin'] }
      }
    ]
  },

  // dataset
  {
    path: '/dataset',
    component: Layout,
    redirect: '/dataset/list',
    alwaysShow: true,
    name: 'Dataset',
    meta: {
      title: 'dataset',
      icon: 'el-icon-s-data',
      roles: ['admin']
    },
    children: [
      {
        path: 'list',
        component: () => import('@/views/dataset/DatasetList'),
        name: 'DatasetList',
        meta: { title: 'datasetList' }
      }
    ]
  },

  // elastic job
  {
    path: '/job',
    component: Layout,
    redirect: '/job/list',
    alwaysShow: true,
    name: 'Job',
    meta: {
      title: 'job',
      icon: 'el-icon-notebook-1',
      roles: ['admin']
    },
    children: [
      {
        path: 'list',
        component: () => import('@/views/job/JobList'),
        name: 'JobList',
        meta: { title: 'jobList' }
      },
      {
        path: 'cost',
        component: () => import('@/views/job/JobCost'),
        name: 'JobCost',
        hidden: true,
        meta: { title: 'jobCost' }
      }
    ]
  },

  // 404 page must be placed at the end !!!
  { path: '*', redirect: '/404', hidden: true }
]

const createRouter = () => new Router({
  // mode: 'history', // require service support
  scrollBehavior: () => ({ y: 0 }),
  routes: constantRoutes
})

const router = createRouter()

// Detail see: https://github.com/vuejs/vue-router/issues/1234#issuecomment-357941465
export function resetRouter() {
  const newRouter = createRouter()
  router.matcher = newRouter.matcher // reset router
}

export default router
