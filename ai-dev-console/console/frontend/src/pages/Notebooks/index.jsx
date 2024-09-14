import React, { useState, useEffect } from 'react';
import {Button, Tooltip, Menu, Space, Dropdown, Select, Form} from 'antd';
import { PageHeaderWrapper } from '@ant-design/pro-layout'
import { connect } from "dva";
import {useIntl, history, getLocale} from "umi";
import {DeleteOutlined, ExclamationCircleFilled, MoreOutlined} from '@ant-design/icons';
import ProTable from '@ant-design/pro-table';
import { listNotebook, createNotebook, stopNotebook, startNotebook, deleteNotebook, syncNotebooks} from './service';
import {queryCurrentUser} from "@/services/global";
import styles from "./style.less";
import LogModal from "./LogModal";

const Notebooks = props => {
  const {
    globalConfig,
    dispatch
  } = props

  const intl = useIntl();
  const toChangeNamespace = (v) => {
    setCurNamespace(v)
  };

  const toCreatNotebook = function(){
    if (notebookInterval !== -1) {
      clearInterval(notebookInterval)
      setNotebookInterval(-1)
    }
    history.push({
      pathname: `/notebooks/notebook-create`,
      query: {}
    });
  }

  const [notebookInterval, setNotebookInterval] = useState(-1);
  const [loading, setLoading] = useState(true);
  const [pvcMenu, setPvcMenu] = useState(<Menu></Menu>);
  const [namespaces, setNamespaces] = useState([]);
  const [curNamespace, setCurNamespace] = useState("");
  const [notebookList, setNotebookList] = useState([]);
  const [usersInfo, setUsersInfo] = useState({});
  useEffect(() => {
    initNamespace()
  }, [])

  const initNamespace = async () => {
    const data = await fetchNamespace()
    setNamespaces(data)
    syncNotebooks(data).then()
  }

  useEffect(() => {
    if (!dispatch || !namespaces || namespaces.length < 1) {
      setLoading(false)
      return
    }
    fetchNotebooks().then()
    if (notebookInterval !== -1) {
      clearInterval(notebookInterval)
      setCurNamespace("")
    }
    if (curNamespace === "") {
      const interval = setInterval(function (){getNotebook(namespaces).then()}, 60000);
      setNotebookInterval(interval)
    }

    setLoading(false)
  }, [namespaces])

  useEffect(() => {
    if (curNamespace === "") {
      setLoading(false)
      return
    }
    fetchNotebooks().then()
    if (notebookInterval !== -1) {
      clearInterval(notebookInterval)
    }
    const interval = setInterval(function (){getNotebook([curNamespace]).then()}, 60000);
    setNotebookInterval(interval)
    setLoading(false)
  }, [curNamespace])

  const fetchNotebooks = async () => {
    if (curNamespace === "") {
      await getNotebook(namespaces)
      setLoading(false)
      return
    }else {
      await getNotebook([curNamespace])
      setLoading(false)
    }
  }

  const getNotebook = async (namespaces) => {
    setLoading(true)
    const response = await listNotebook(namespaces, usersInfo.loginName, usersInfo.loginId)
    if (response && response.code === '200') {
      if (response.data === null || response.data.length < 1) {
        setNotebookList([])
        setLoading(false)
        return
      }
      response.data.map(x=>{x.key=x.name+'.'+x.namespace; x.url=window.location.origin + x.accessPath; x.namespace=x.namespace;})
      //response.data.map(x=>{x.key=x.name; x.url="http://120.76.245.247:8080" + x.accessPath; x.namespace=namespace})
      //response.data.map(x=>{x.key=x.name; x.url="http://127.0.0.1:8080" + x.accessPath; x.namespace=namespace})
      setNotebookList(response.data)
    }
    setLoading(false)
  }

  const updatePvcMenu = (pvcs) => {
    if (!pvcs) {
      return setPvcMenu(<Menu></Menu>);
    }
    let menuItems = pvcs.map((x) => <Menu.Item key={x}>{x}</Menu.Item>);
    let newMenu = (<Menu>{menuItems}</Menu>);
    setPvcMenu(newMenu)
  }

  const onDeleteNotebook = async (namespace, name) => {
    const response = await deleteNotebook(namespace, name)
    if (response.code == 200) {
      setNotebookList(notebookList.filter(item=>item.name !== name || item.namespace != namespace))
    }
  }

  const fetchNamespace = async () => {
    let currentUser = await queryCurrentUser();
    const userInfos = currentUser.data && currentUser.data.loginId ? currentUser.data : {};
    setUsersInfo(userInfos);
    const newNamespaces = [];
    if(userInfos && userInfos.namespaces) {
      for(let idx in userInfos.namespaces) {
        newNamespaces.push(userInfos.namespaces[idx])
      }
    }
    return newNamespaces
  }

  const columns = [
    {
      title: intl.formatMessage({id: 'dlc-notebook-name'}),
      dataIndex: 'name',
      render: (_, record) => {
        if (record.status == "Running") {
          return <a target="_blank" href={record.url}>{_}</a>
        }else if (record.status == "Stopped"){
          return (
              <Tooltip title={record.event} >
                <a className="disabled" target="_blank">{_}</a>
              </Tooltip>)
        }else{
          return ( <Tooltip title={record.errMessage} >
            <a className="disabled" target="_blank">{_}</a>
          </Tooltip>)
        }
        },
    },
    {
      title: intl.formatMessage({id: 'dlc-notebook-namespace'}),
      dataIndex: 'namespace',
    },
    {
      title: intl.formatMessage({id: 'dlc-notebook-status'}),
      dataIndex: 'status',
      valueEnum: {
        closed: { text: '关闭', status: 'Default' },
        running: { text: '运行', status: 'Success' },
        error: { text: '异常', status: 'Error' },
      },
    },
    {
      title: intl.formatMessage({id: 'dlc-notebook-age'}),
      dataIndex: 'age',
    },
    {
      title: intl.formatMessage({id: 'dlc-notebook-image'}),
      dataIndex: 'image',
    },
    {
      title: 'GPUs',
      dataIndex: 'gpus',
    },
    {
      title: 'CPUs',
      dataIndex: 'cpus',
    },
    {
      title: intl.formatMessage({id: 'dlc-notebook-memory'}),
      dataIndex: 'memory',
    },
    {
      title: 'Token',
      dataIndex: 'token',
    },
    {
      title: 'PVCs',
      dataIndex: 'volumes',
      render: (_, record) => {
        return (
            <Tooltip title="volume details">
              <Space direction="vertical">
                <Space wrap>
                  <Dropdown overlay={pvcMenu} placement="bottomLeft" trigger={['click']}>
                    <Button icon={<MoreOutlined />} onClick={()=>updatePvcMenu(record.volumes)}/>
                  </Dropdown>
                </Space>
              </Space>
            </Tooltip>
        );
      }
    },
    {
      title: intl.formatMessage({id: 'dlc-notebook-operate'}),
      width: 180,
      key: 'option',
      valueType: 'option',
      render: (_, record) => [
        <Tooltip key="delete" title="delete"><Button shape="circle" icon={<DeleteOutlined />} onClick={()=>onDeleteNotebook(record.namespace, record.name)} /></Tooltip>,
      ],
    },
  ]

  return (
      <PageHeaderWrapper title={<></>}>
        <ProTable
            loading={loading}
            columns={columns}
            dataSource={notebookList}
            //request={(params, sorter, filter) => {
            //  // 表单搜索项会从 params 传入，传递给后端接口。
            //  console.log('request:', params, sorter, filter);
            //  return Promise.resolve({
            //    data: data,
            //    success: true,
            //  });
            //}}
            options={{
              fullScreen: true,
              setting: true,
              reload: () => fetchNotebooks(),
            }}
            toolBarRender={() => [
              (<Select
                  onChange={toChangeNamespace}
                  className={styles.namespaces}
                  placeholder={intl.formatMessage({id: 'dlc-notebook-change-namespace'})}
              >
                {namespaces.map(data => (
                    data &&
                    <Select.Option title={data} value={data} key={data}>
                      {data}
                    </Select.Option>
                ))}
              </Select>),
              (<Button type="primary" key="primary" onClick={()=>toCreatNotebook()}>
                {intl.formatMessage({id: 'dlc-notebook-create'})}
              </Button>)
            ]}
            rowKey="key"
            pagination={{
              total: 10,
              showQuickJumper: true,
            }}
            search={false}
            dateFormatter="string"
        />
      </PageHeaderWrapper>
  );
}


export default connect(({ global }) => ({
  globalConfig: global.config
}))(Notebooks);