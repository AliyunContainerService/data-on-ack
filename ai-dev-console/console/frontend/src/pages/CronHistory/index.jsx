import React, { useRef, useState, useEffect, Fragment } from "react";
import { PageHeaderWrapper } from "@ant-design/pro-layout";
import ProTable from "@ant-design/pro-table";
import {
  queryCronHistory,
} from "./service";
import moment from "moment";
import { connect, useIntl, history } from "umi";
import { queryCurrentUser } from "@/services/global";
const TableList = ({ globalConfig }) => {
  const intl = useIntl();
  const [loading, setLoading] = useState(true);
  const [cronHistories, setCronHistories] = useState([]);
  const [total, setTotal] = useState(0);
  const [users, setUsers] = useState({});

  const pageSizeRef = useRef(20);
  const currentRef = useRef(1);
  const paramsRef = useRef({});
  const fetchIntervalRef = useRef();
  const actionRef = useRef();
  const formRef = useRef();

  const searchInitialParameters = {
    current: 1,
    page_size: 20,
  };

  useEffect(() => {
    fetchCronHistory();
    fetchUser();
    const interval = 10 * 1000;
    fetchIntervalRef.current = setInterval(() => {
      fetchCronHistorySilently();
    }, interval);
    return () => {
      clearInterval(fetchIntervalRef.current);
    };
  }, []);

  const fetchCronHistory = async () => {
    setLoading(true);
    await fetchCronHistorySilently();
    setLoading(false);
  };

  const fetchUser = async () => {
    const users = await queryCurrentUser();
    let userInfos = users.data ? users.data : {};
    setUsers(userInfos);
  };

  const fetchCronHistorySilently = async () => {
    let queryParams = { ...paramsRef.current };
    queryParams = {
      ...queryParams,
      ...searchInitialParameters,
    };

    let params = history.location.query;

    let cronHistories = await queryCronHistory({
      name: params.cron_name,
      namespace: params.namespace,
      job_name: queryParams.name,
      job_status: queryParams.jobStatus === "All" ? undefined : queryParams.jobStatus,
    });
    setCronHistories(cronHistories.data);
    setTotal(cronHistories.total);
  };

  const onDetail = (job) => {
    history.push({
      pathname: `/jobs/detail`,
      query: {
        id: job.id,
        region: job.deployRegion,
        start_date: moment(job.createTime)
            .utc()
            .format("YYYY-MM-DD"),
        job_name: job.name,
        namespace: job.namespace,
        kind: job.jobType,
        current_page: 1,
        page_size: 10,
        is_cron: true,
      },
    });
  };

  const onSearchSubmit = (params) => {
    paramsRef.current = params;
    fetchCronHistory();
  };

  const onTableChange = (pagination) => {
    if (pagination) {
      currentRef.current = pagination.current;
      pageSizeRef.current = pagination.pageSize;
      fetchCronHistory();
    }
  };

  let columns = [
    {
      // title: 'Namespace',
      title: intl.formatMessage({ id: "dlc-dashboard-namespace" }),
      dataIndex: "namespace",
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: "dlc-dashboard-job-type" }),
      dataIndex: "jobType",
      hideInSearch: true,
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
      dataIndex: "jobStatus",
      initialValue: searchInitialParameters.jobStatus,
      valueEnum: {
        All: {
          text: intl.formatMessage({ id: "dlc-dashboard-all" }),
          // text: 'All',
          status: "Default",
        },
        Created: {
          text: intl.formatMessage({ id: "dlc-dashboard-has-created" }),
          // text: 'Created',
          status: "Default",
        },
        Waiting: {
          text: intl.formatMessage({ id: "dlc-dashboard-waiting-for" }),
          // text: 'Waiting',
          status: "Processing",
        },
        Running: {
          text: intl.formatMessage({ id: "dlc-dashboard-executing" }),
          // text: 'Running',
          status: "Processing",
        },
        Succeeded: {
          text: intl.formatMessage({ id: "dlc-dashboard-execute-success" }),
          // text: 'Succeeded',
          status: "Success",
        },
        Failed: {
          text: intl.formatMessage({ id: "dlc-dashboard-execute-failure" }),
          // text: 'Failed',
          status: "Error",
        },
        Stopped: {
          text: intl.formatMessage({ id: "dlc-dashboard-has-stopped" }),
          // text: 'Stopped',
          status: "Error",
        },
      },
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
      dataIndex: "endTime",
      //valueType: "date",
      hideInSearch: true,
    },
    {
      width: 142,
      title: intl.formatMessage({ id: "dlc-dashboard-execution-time" }),
      dataIndex: "durationTime",
      hideInSearch: true,
      render: (text) => <Fragment>{text && text.split(".")[0]}</Fragment>,
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
        dataSource={cronHistories}
        onSubmit={(params) => onSearchSubmit(params)}
        headerTitle={intl.formatMessage({ id: "dlc-dashboard-cron-list" })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey={(record, index) => index}
        columns={[...cronName, ...columns]}
        options={{
          fullScreen: true,
          setting: true,
          reload: () => fetchCronHistory(),
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
