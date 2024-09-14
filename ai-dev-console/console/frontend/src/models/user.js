import { queryCurrentUser, queryUserByToken, loginByRam } from '@/services/global';
import { queryLogin, queryLoginOut } from '@/services/login';
import { history } from 'umi';
import { message } from 'antd';

const UserModel = {
  namespace: 'user',
  state: {
    currentUser: {},
    ssoRedirect: undefined,
    userLogin: {},
  },
  effects: {
    * loginByRam({ payload }, { call, put }) {
      const response = yield call(loginByRam);
      if (response && response.code !== "200") { // 401: StatusUnauthorized
        yield payload = response
        yield put({
          type: 'redirect',
          payload,
        });
      } else {
        yield put({
          type: 'saveCurrentUser',
          payload: response,
        });
      }
    },

    * fetchCurrent({ payload }, { call, put }) {
      const response = yield call(queryCurrentUser);
      if (response && response.code !== "200") { // 401: StatusUnauthorized
        yield payload = response
        yield put({
          type: 'redirect',
          payload,
        });
      } else {
        yield put({
          type: 'saveCurrentUser',
          payload: response,
        });
      }
    },
    * fetchUserByToken({ payload }, { call, put }) {
      const response = yield call(queryUserByToken, payload);
      if (response && response.code !== "200") { // 401: StatusUnauthorized
        yield payload = response
        yield put({
          type: 'redirect',
          payload,
        });
      } else {
        yield put({
          type: 'saveCurrentUser',
          payload: response,
        });
      }
    },
    * fetchLogin({ payload }, { call, put }) {
      const response = yield call(queryLogin.bind(null, null));
      if (response && response.code !== "200") {
        yield payload = response
        yield put({
          type: 'redirect',
          payload: response,
        });
      } else {
        yield put({
          type: 'loginSuccess',
          payload: response || { data: { msg: null, success: null } },
        });
      }
    },
    * fetchLoginOut({ payload }, { call, put }) {
      const response = yield call(queryLoginOut);
      if (response && response.code !== "200") {
        yield payload = response
        yield put({
          type: 'redirect',
          payload,
        });
      } else {
        yield put({
          type: 'loginOut',
          payload: response || {},
          currentUser: {}
        });
      }
    },
  },

  reducers: {
    redirect(state, action) {
      return { ...state, ssoRedirect: action.payload.data.ssoRedirect, currentUser: {} };
    },
    saveCurrentUser(state, action) {
      return {
        ...state,
        ssoRedirect: undefined,
        currentUser: action.payload.data || {},
      };
    },
    loginSuccess(state, { payload: { data: { msg, success } } }) {
      if (success === 'true') {
        window.setTimeout(() => {
          history.push('/cluster');
        }, 2000);
      } else {
        message.error(msg || 'Login exception');
      }
      return { ...state, userLogin: payload };
    },
    loginOut(state, { action, currentUser }) {
      return { ...state, currentUser: {}, userLogin: "" };
    },
  },
};
export default UserModel;
