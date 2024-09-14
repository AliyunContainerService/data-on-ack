import {
  ExclamationCircleOutlined,
  DeleteOutlined,
  PauseOutlined,
  PlayCircleOutlined,
} from "@ant-design/icons";
import { Modal, message, Tooltip } from "antd";
import React, { useRef, useState, useEffect, Fragment } from "react";
import { PageHeaderWrapper } from "@ant-design/pro-layout";
import ProTable from "@ant-design/pro-table";
import {
  queryCrons,
  deleteCron,
  suspendCron,
  resumeCron,
} from "./service";
import moment from "moment";
import { connect, useIntl, history } from "umi";
import { queryCurrentUser } from "@/services/global";
const TableList = ({ globalConfig }) => {
  const intl = useIntl();
  const [loading, setLoading] = useState(true);
  const [crons, setCrons] = useState([]);
  const [total, setTotal] = useState(0);
  const [users, setUsers] = useState({});

  const pageSizeRef = useRef(20);
  const currentRef = useRef(1);
  const paramsRef = useRef({});
  const fetchIntervalRef = useRef();
  const actionRef = useRef();
  const formRef = useRef();

  const searchInitialParameters = {
    status: "All",
    submitDateRange: [moment().subtract(30, "days"), moment()],
    current: 1,
    page_size: 20,
  };

  useEffect(() => {
    fetchCrons();
    fetchUser();
    const interval = 10 * 1000;
    fetchIntervalRef.current = setInterval(() => {
      fetchCronsSilently();
    }, interval);
    return () => {
      clearInterval(fetchIntervalRef.current);
    };
  }, []);

  const fetchCrons = async () => {
    setLoading(true);
    await fetchCronsSilently();
    setLoading(false);
  };

  const fetchUser = async () => {
    const users = await queryCurrentUser();
    let userInfos = users.data ? users.data : {};
    setUsers(userInfos);
  };

  const fetchCronsSilently = async () => {
    let queryParams = { ...paramsRef.current };
    if (!paramsRef.current.submitDateRange) {
      queryParams = {
        ...queryParams,
        ...searchInitialParameters,
      };
    }
    let crons = await queryCrons({
      name: queryParams.name,
      // namespace: globalConfig.namespace,
      status:
        queryParams.status === "All" ? undefined : queryParams.status,
      start_time: moment(queryParams.submitDateRange[0])
        .hours(0)
        .minutes(0)
        .seconds(0)
        .utc()
        .format(),
      end_time: moment(queryParams.submitDateRange[1])
        .hours(0)
        .minutes(0)
        .seconds(0)
        .add(1, "days")
        .utc()
        .format(),
      current_page: currentRef.current,
      kind: queryParams.type,
      page_size: pageSizeRef.current,
    });
    setCrons(crons.data);
    setTotal(crons.total);
  };

  const onDetail = (cron) => {
    history.push({
      pathname: `/crons/history`,
      query: {
        cron_name: cron.name,
        namespace: cron.namespace,
        kind: cron.type,
        current_page: 1,
        page_size: 10,
      },
    });
  };

  const onCronDelete = (cron) => {
    Modal.confirm({
      title: intl.formatMessage({ id: "dlc-dashboard-delete-job" }),
      icon: <ExclamationCircleOutlined />,
      content: `${intl.formatMessage({
        id: "dlc-dashboard-delete-job-confirm",
      })} ${cron.name} ?`,
      onOk: () =>
        deleteCron(
          cron.namespace,
          cron.name,
          cron.id,
        ).then(() => {
          const { current } = actionRef;
          if (current) {
            current.reload();
          }
        }),
      onCancel() {},
    });
  };

  const onCronSuspend = (cron) => {
    Modal.confirm({
      title: intl.formatMessage({ id: "dlc-dashboard-cron-suspend" }),
      icon: <ExclamationCircleOutlined />,
      content: `${intl.formatMessage({
        id: "dlc-dashboard-cron-suspend-confirm",
      })} ${cron.name} ?`,
      onOk: () =>
          suspendCron(
              cron.namespace,
              cron.name,
              cron.id,
          ).then(() => {
            const { current } = actionRef;
            if (current) {
              current.reload();
            }
          }),
      onCancel() {},
    });
  };

  const onCronResume = (cron) => {
    Modal.confirm({
      title: intl.formatMessage({ id: "dlc-dashboard-cron-resume" }),
      icon: <ExclamationCircleOutlined />,
      content: `${intl.formatMessage({
        id: "dlc-dashboard-cron-resume-confirm",
      })} ${cron.name} ?`,
      onOk: () =>
          resumeCron(
              cron.namespace,
              cron.name,
              cron.id,
          ).then(() => {
            const { current } = actionRef;
            if (current) {
              current.reload();
            }
          }),
      onCancel() {},
    });
  };

  const onSearchSubmit = (params) => {
    paramsRef.current = params;
    fetchCrons();
  };

  const onTableChange = (pagination) => {
    if (pagination) {
      currentRef.current = pagination.current;
      pageSizeRef.current = pagination.pageSize;
      fetchCrons();
    }
  };
  const Tip = ({ dlc, Click, disabled, IconComponent, visible}) => {
    return (
        <span>
          {
            visible && (
                <Tooltip title={intl.formatMessage({ id: dlc })}>
                  <a onClick={() => Click()} disabled={disabled}>
                    {IconComponent}
                  </a>
                </Tooltip>
            )
          }
        </span>
    );
  };

  let columns = [
    {
      // title: 'Date Range',
      title: intl.formatMessage({ id: "dlc-dashboard-time-interval" }),
      dataIndex: "submitDateRange",
      valueType: "dateRange",
      initialValue: searchInitialParameters.submitDateRange,
      hideInTable: true,
    },
    {
      // title: 'Namespace',
      title: intl.formatMessage({ id: "dlc-dashboard-namespace" }),
      dataIndex: "namespace",
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: "dlc-dashboard-job-type" }),
      dataIndex: "type",
      valueEnum: {
        PyTorchJob: {
          text: "PyTorchJob",
          status: "Default",
        },
        TFJob: {
          text: "TFJob",
          status: "Default",
        },
      },
    },
    {
      // title: 'Status',
      title: intl.formatMessage({ id: "dlc-dashboard-status" }),
      width: 128,
      dataIndex: "status",
      initialValue: searchInitialParameters.status,
      valueEnum: {
        All: {
          text: intl.formatMessage({ id: "dlc-dashboard-all" }),
          // text: 'All',
          status: "Default",
        },
        Running: {
          text: intl.formatMessage({ id: "dlc-dashboard-cron-running" }),
          // text: 'Running',
          status: "Running",
        },
        Suspend: {
          text: intl.formatMessage({ id: "dlc-dashboard-cron-suspend" }),
          // text: 'Waiting',
          status: "Suspend",
        },
      },
    },
    {
      width: 142,
      title: intl.formatMessage({ id: "dlc-dashboard-cron-schedule" }),
      dataIndex: "schedule",
      hideInSearch: true,
      render: (text) => <Fragment>{text && text.split(".")[0]}</Fragment>,
    },
    {
      // title: 'Create Time',
      title: intl.formatMessage({ id: "dlc-dashboard-creation-time" }),
      dataIndex: "createTime",
      //valueType: "date",
      hideInSearch: true,
    },
    {
      // title: 'End Time',
      title: intl.formatMessage({ id: "dlc-dashboard-end-time" }),
      dataIndex: "deadline",
      //valueType: "date",
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: "dlc-dashboard-operation" }),
      dataIndex: "option",
      valueType: "option",
      render: (_, record) => {
        let isDisabled = true;
        // if (users.accountId === users.loginId) {
        //   isDisabled = true;
        // } else {
        //   isDisabled = record.jobUserId && record.jobUserId === users.loginId;
        // }

        return (
          <Fragment>
            <Tip
                dlc={"dlc-dashboard-cron-suspend"}
                Click={onCronSuspend.bind(this, record)}
                disabled={!isDisabled}
                visible={record.status === "Running" }
                IconComponent={
                  <PauseOutlined
                      style={{
                        marginRight: "10px",
                        color: isDisabled ? "#1890ff" : "",
                      }}
                  />
                }
            />
            <Tip
                dlc={"dlc-dashboard-cron-resume"}
                Click={onCronResume.bind(this, record)}
                disabled={!isDisabled}
                visible={record.status === "Suspend" }
                IconComponent={
                  <PlayCircleOutlined
                      style={{
                        marginRight: "10px",
                        color: isDisabled ? "#1890ff" : "",
                      }}
                  />
                }
            />
            <Tip
              dlc={"dlc-dashboard-delete"}
              Click={onCronDelete.bind(this, record)}
              disabled={!isDisabled}
              visible={true}
              IconComponent={
                <DeleteOutlined
                  style={{ color: isDisabled ? "#d9363e" : "" }}
                />
              }
            />
          </Fragment>
        );
      },
    },
  ];
  let cronName = [
    {
      title: intl.formatMessage({ id: "dlc-dashboard-name" }),
      dataIndex: "name",
      width: 196,
      render: (_, r) => {
        return <a onClick={() => onDetail(r)}>{r.name}</a>;
      },
    },
  ];
  return (
    <PageHeaderWrapper title={<></>}>
      <ProTable
        loading={loading}
        dataSource={crons}
        onSubmit={(params) => onSearchSubmit(params)}
        headerTitle={intl.formatMessage({ id: "dlc-dashboard-cron-list" })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey={(record, index) => index}
        columns={[...cronName, ...columns]}
        options={{
          fullScreen: true,
          setting: true,
          reload: () => fetchCrons(),
        }}
        onChange={onTableChange}
        pagination={{ total: total }}
        scroll={{ y: 450 }}
      />

    </PageHeaderWrapper>
  );
};

export default connect(({ global }) => ({
  globalConfig: global.config,
}))(TableList);
