import React from "react";
import { PageLoading } from "@ant-design/pro-layout";
import { connect, history, Redirect } from "umi";
import { LoginForm, ProFormText } from '@ant-design/pro-form';
import { LockOutlined } from "@ant-design/icons";
import { Tabs, Space, Button } from 'antd';

class SecurityLayout extends React.Component {
  state = {
    isReady: false,
  }

  componentDidMount() {
    const { dispatch } = this.props;
    if (dispatch) {
      dispatch({
        type: "user/fetchCurrent"
      });
    }
    this.setState({
      isReady: true
    });
  }

  render() {
    const { isReady } = this.state;
    const { children, loading, currentUser } = this.props; // You can replace it to your authentication rule (such as check token exists)
    // 你可以把它替换成你自己的登录认证规则（比如判断 token 是否存在）
    const isLogin = currentUser && currentUser.accountId;

    if ((!isLogin && loading) || !isReady) {
      return <PageLoading />
    }

    if (isLogin) {
      return children
    } else {
      window.location.href = "/login"
      return <PageLoading />
    }
  }

}

export default connect(({ user, loading }) => ({
  currentUser: user.currentUser,
  ssoRedirect: user.ssoRedirect,
  loading: loading.models.user
}))(SecurityLayout);
