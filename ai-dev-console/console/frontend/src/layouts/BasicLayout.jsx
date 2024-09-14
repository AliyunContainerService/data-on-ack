/**
 * Ant Design Pro v4 use `@ant-design/pro-layout` to handle Layout.
 * You can view component api by:
 * https://github.com/ant-design/ant-design-pro-layout
 */
import ProLayout from "@ant-design/pro-layout";
import React, { useEffect, useState } from "react";
import { Link, getLocale, setLocale, formatMessage, connect } from "umi";
import {
  MenuUnfoldOutlined,
  MenuFoldOutlined
} from "@ant-design/icons";
import { Result, Button, Select } from "antd";
import Authorized from "@/utils/Authorized";
import RightContent from "@/components/GlobalHeader/RightContent";
import AvatarDropdown from '@/components/GlobalHeader/AvatarDropdown'
import { getAuthorityFromRouter } from "@/utils/utils";
import { PageLoading } from "@ant-design/pro-layout";
import logo from "../assets/logo.svg";
import { useIntl } from "umi";
import "./index.less";
import { ModelRegistryService } from "@/pages/ModelManage/services";
const noMatch = (
  <Result
    status="403"
    title="403"
    subTitle="Sorry, you are not authorized to access this page."
    extra={
      <Button type="primary">
        <Link to="/user/login">Go Login</Link>
      </Button>
    }
  />
);
const { Option } = Select;

const BasicLayout = props => {
  const {
    dispatch,
    children,
    settings,
    collapsed,
    config,
    configLoading,
    location = {
      pathname: "/"
    }
  } = props;

  const intl = useIntl();

  const [namespaceValue, setNamespaceValue] = useState('');
  useEffect(() => {
    if (dispatch) {
      dispatch({
        type: "global/fetchNamespaces"
      });
      dispatch({
        type: "global/fetchConfig"
      });
      handleMenuCollapse(true)
    }
  }, []);

  useEffect(() => {
    if (sessionStorage.getItem('namespace')) {
      setNamespaceValue(sessionStorage.getItem('namespace'));
    } else {
      if (config) {
        setNamespaceValue(config.namespace);
      }
    }
  }, [config]);

  const [pipelineInstall, setPipelineInstall] = useState(false)
  useEffect(() => {
    async function fetchPipelineInstall() {
      let response = await fetch('/pipelineCheckInstall')
      let response_json = await response.json()
      setPipelineInstall(response_json.data.install)
    }
    fetchPipelineInstall()
  })

  // Check whether Mlflow is healthy
  const [mlflowHealth, setMlflowHealth] = useState(false)
  useEffect(() => {
    ModelRegistryService.checkHealth().then((res) => {
      if (res.ok) {
        setMlflowHealth(true)
      } else {
        setMlflowHealth(false)
      }
    }).catch(() => {
      setMlflowHealth(false)
    })
  })

  /**
   * init variables
   */

  const handleMenuCollapse = payload => {
    if (dispatch) {
      dispatch({
        type: "global/changeLayoutCollapsed",
        payload: !props.collapsed
      });
    }
  }; // get children authority

  const onChangeNamespace = v => {
    sessionStorage.setItem("namespace", v);
    // setNamespaceValue(sessionStorage.getItem('namespace'));
    window.location.reload();
  };

  const authorized = getAuthorityFromRouter(
    props.route.routes,
    location.pathname || "/"
  ) || {
    authority: undefined
  };

  if (configLoading || !config) {
    return <PageLoading />;
  }
  const changLang = () => {
    const locale = getLocale();
    if (!locale || locale === 'zh-CN' || locale === 'en') {
      setLocale('en-US', true);
    } else {
      setLocale('zh-CN', true);
    }
  };


  return (
    <ProLayout
      logo={logo}
      breakpoint="xxl"
      formatMessage={formatMessage}
      title={<div style={{ position: "relative" }}>Cloud Native AI
        {/* <br/><span style={{position:"absolute",fontSize:"14px"}}>{config.version}</span> */}
      </div>}
      menuHeaderRender={(logoDom, titleDom) => (
        <Link to="/">
          {/* {logoDom} */}
          {titleDom}
        </Link>
      )}
      onCollapse={handleMenuCollapse}
      menuDataRender={(menuData) => {
        return menuData.map(menuDataItem => {
          if (menuDataItem.name === intl.formatMessage({id: "Model Manage"})) {
            if (!mlflowHealth) {
              menuDataItem.hideInMenu = true;
            }
          }

          if (menuDataItem.name === "Kubeflow Pipelines") {
            if (!pipelineInstall) {
              menuDataItem.hideInMenu = true
            }
            menuDataItem.path = window.location.origin + "/pipeline/"
          }

          const localItem = {
            ...menuDataItem,
            // children: menuDataItem.children ? menuDataRender(menuDataItem.children) : []
          };

          // Use Authorized check all menu item
          return Authorized.check(menuDataItem.authority, localItem, null);
        });
      }}
      menuItemRender={(menuItemProps, defaultDom) => {
        if (
          menuItemProps.isUrl ||
          menuItemProps.children ||
          !menuItemProps.path
        ) {
          return defaultDom;
        }

        return <Link to={menuItemProps.path}>{defaultDom}</Link>;
      }}
      // breadcrumbRender={(routers = []) => [
      //   {
      //     path: "/",
      //     breadcrumbName: "KubeDL"
      //   },
      //   ...routers
      // ]}
      itemRender={(route, params, routes, paths) => {
        return <Link to={route.path}>{route.breadcrumbName}</Link>;
      }}
      // footerRender={footerRender}
      rightContentRender={() => <RightContent />}
      headerRender={() => (
        <>
          <div style={{
            position: 'absolute',
            right: '30px'
          }}>
            <span style={{ marginRight: '10px' }}>
              <AvatarDropdown />
            </span>
            <Select defaultValue={getLocale() === 'zh-CN' || getLocale() === 'en' ? 'chinese' : 'english'} style={{ width: 120 }} onChange={changLang}>
              <Option value="chinese">简体中文</Option>
              <Option value="english">English</Option>
            </Select>
          </div>
          {React.createElement(
            props.collapsed ? MenuUnfoldOutlined : MenuFoldOutlined,
            {
              className: "trigger",
              onClick: handleMenuCollapse
            }
          )}
          {/*<Select*/}
          {/*  style={{ width: 225 }}*/}
          {/*  value={namespaceValue}*/}
          {/*  placeholder="请选择集群ID"*/}
          {/*  onChange={onChangeNamespace}*/}
          {/*>*/}
          {/*  {props.namespaces.length > 0 && props.namespaces.map((item) => <Option key={item.value} value={item.value}>{item.label}</Option>)}*/}
          {/*</Select>*/}
        </>
      )}
      {...props}
      {...settings}
    >
      <Authorized authority={authorized.authority} noMatch={noMatch}>
        {children}
      </Authorized>
    </ProLayout>
  );
};
export default connect(({ global, settings, loading }) => ({
  collapsed: false,
  config: global.config,
  namespaces: global.namespaces,
  configLoading: loading.effects["global/fetchConfig"],
  settings
}))(BasicLayout);
