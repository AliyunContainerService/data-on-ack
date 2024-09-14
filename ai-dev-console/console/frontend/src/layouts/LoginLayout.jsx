import React from "react";
import { PageLoading } from "@ant-design/pro-layout";
import { connect } from "umi";
import { LoginForm, ProFormText } from '@ant-design/pro-form';
import { LockOutlined } from "@ant-design/icons";
import { Tabs, Space, Button } from 'antd';

class LoginLayout extends React.Component {
    state = {
        isReady: false,
        login: true,
        loginType: 'token',
        token: ''
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

    setLoginType = (lgType) => {
        this.setState({ loginType: lgType })
    }

    setToken = (event) => {
        const { name, value } = event.target
        this.setState({ token: value }) //async set
    }

    onLogin = () => {
        const { isReady } = this.state;
        const { children, loading, currentUser, ssoRedirect } = this.props; // You can replace it to your authentication rule (such as check token exists)
        const isLogin = currentUser && currentUser.accountId
        if ((!isLogin && loading) || !isReady) {
            return <PageLoading />
        }

        if (isLogin) {
            window.location.href = "/"
            return
        }

        if (ssoRedirect) {
            window.location.href = ssoRedirect
            return
        }

        if (this.state.loginType === 'ram') {
            const { dispatch } = this.props;
            if (dispatch) {
                dispatch({
                    type: "user/loginByRam"
                });
            }
            this.setState({
                isReady: true
            });
        } else {
            const { dispatch } = this.props;
            if (dispatch) {
                dispatch({
                    type: "user/fetchUserByToken",
                    payload: this.state.token
                })
            }
            this.setState({
                isReady: true
            })
        }
    }

    render() {
        const { isReady, login } = this.state;
        const { children, loading, currentUser, ssoRedirect } = this.props; // You can replace it to your authentication rule (such as check token exists)
        // 你可以把它替换成你自己的登录认证规则（比如判断 token 是否存在）
        const isLogin = currentUser && currentUser.accountId;

        if (!isLogin && ssoRedirect) {
            window.location.href = ssoRedirect
            return <PageLoading/>
        }

        if ((!isLogin && loading) || !isReady) {
            return <PageLoading />
        }

        if (isLogin) {
            window.location.href = "/"
            return <PageLoading/>
        }

        return <div style={{ backgroundColor: 'white' }}>
            <LoginForm
                logo="https://img.alicdn.com/tfs/TB13DzOjXP7gK0jSZFjXXc5aXXa-212-48.png"
                title="Develop Console"
                subTitle="ACK Cloud Native AI"
                actions={
                    <Space>
                    </Space>
                }
                submitter={{
                    resetButtonProps: {
                        style: {
                            // 隐藏重置按钮
                            display: 'none',
                        },
                    },
                    searchConfig: { submitText: 'Login', },
                    render: (props, doms) => {
                        return [
                            <Button type="primary" key="submit" block onClick={this.onLogin}>
                                Login
                            </Button>
                        ]
                    }
                }
                }
            >
                <Tabs activeKey={this.state.loginType} onChange={(activeKey) => this.setLoginType(activeKey)}>
                    <Tabs.TabPane key={'token'} tab={'K8s token'} />
                    <Tabs.TabPane key={'ram'} tab={'Aliyun Ram'} />
                </Tabs>
                {
                    this.state.loginType === 'token' && (
                        <>
                            <ProFormText.Password
                                name="token"
                                fieldProps={{
                                    size: 'large',
                                    prefix: <LockOutlined className={'prefixIcon'} />,
                                }}
                                value={this.state.token}
                                onChange={this.setToken}
                                placeholder={'Token'}
                                rules={[
                                    {
                                        required: true,
                                        message: 'input token from k8s',
                                    },
                                ]}
                            />
                        </>
                    )
                }
                {
                    this.state.loginType === 'ram' && (
                        <>
                            <Space>Click login to redirect to Aliyun Ram </Space>
                        </>
                    )
                }
                <div
                    style={{
                        marginBottom: 24,
                    }}
                >
                    <a
                        style={{
                            float: 'right',
                        }}
                        href="https://help.aliyun.com/document_detail/252770.html"
                    >
                        Login Help
                    </a>
                </div>
            </LoginForm >
        </div >;
    }

}

export default connect(({ user, loading }) => ({
    currentUser: user.currentUser,
    ssoRedirect: user.ssoRedirect,
    loading: loading.models.user
}))(LoginLayout);
