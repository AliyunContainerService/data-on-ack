import {
  LoginOutlined,
  LogoutOutlined,
  SettingOutlined,
  UserOutlined
} from "@ant-design/icons";
import { Avatar, Menu, Spin } from "antd";
import React from "react";
import { history,connect,formatMessage } from "umi";
import HeaderDropdown from "../HeaderDropdown";
import styles from "./index.less";

const notLoggedIn = formatMessage({
  id: 'dlc-dashboard-not-logged-in'
});

class AvatarDropdown extends React.Component {
  onMenuClick = event => {
    const key = event.key;
    const dispatch = this.props.dispatch;
    if(environment && environment !== "eflops" && environment !== "dlc") {
      if (key === "logout") {
        if (dispatch) {
          dispatch({
            type: "login/logout"
          });
        }
        return;
      }
      history.push(`/account/${key}`);
    } else {
      if (key === 'login') {
        dispatch({
          type: "user/fetchCurrent",
          payload: {}
        });
      } else {
        dispatch({
          type: "user/fetchLoginOut",
          payload: {}
        });
      }
      
      window.setTimeout(() => {
        history.push('/');
      }, 1500);
    }
  };

  render() {
    const {
      currentUser = {
        avatar: "",
        name: ""
      },
      menu,
      userLogin
    } = this.props;
    const menuHeaderDropdown = (
      <Menu
        className={styles.menu}
        selectedKeys={[]}
        onClick={this.onMenuClick}
      >
        {menu && (
          <Menu.Item key="center">
            <UserOutlined />
            个人中心
          </Menu.Item>
        )}
        {menu && (
          <Menu.Item key="settings">
            <SettingOutlined />
            个人设置
          </Menu.Item>
        )}
        {menu && <Menu.Divider />}
        {currentUser && currentUser.name && (
          <Menu.Item key="logout">
            <LogoutOutlined />
             退出登录
           </Menu.Item>
        )}
        {(!currentUser || !currentUser.name) && (
          <Menu.Item key="login">
            <LoginOutlined />
             登录
           </Menu.Item>
        )}
      </Menu>
    );
    return currentUser && currentUser.loginName ? 
    (
      <HeaderDropdown overlay={menuHeaderDropdown}>
        <span className={`${styles.action} ${styles.account}`}>
          <Avatar
            size="small"
            icon={<UserOutlined />}
            style={{ color: "#1890ff", marginRight: 12 }}
          />
          <span className={styles.name}>{currentUser.loginName}</span>
        </span>
      </HeaderDropdown>
    ) : (
      <HeaderDropdown overlay={menuHeaderDropdown}>
        <span className={`${styles.action} ${styles.account}`}>
          <Avatar
            size="small"
            icon={<UserOutlined />}
            style={{ color: "#1890ff", marginRight: 12 }}
          />
          <span className={styles.name}>{currentUser.loginName}</span>
        </span>
      </HeaderDropdown>
    );
  }
}

export default connect(({ user }) => ({
  currentUser: user.currentUser,
  userLogin: user.userLogin
}))(AvatarDropdown);
