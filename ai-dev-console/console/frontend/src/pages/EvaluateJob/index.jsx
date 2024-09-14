import {
    ExclamationCircleOutlined,
    FundViewOutlined,
    PlusSquareOutlined,
    FormOutlined,
    DeleteOutlined,
    CopyOutlined,
} from "@ant-design/icons";
import { Modal, message, Tooltip, Select, Button } from "antd";
import React, { useRef, useState, useEffect, Fragment } from "react";
import { PageHeaderWrapper } from "@ant-design/pro-layout";
import ProTable from "@ant-design/pro-table";
import {
    deleteJobs,
    queryJobs,
} from "./service";
import moment from "moment";
import { connect, useIntl, history } from "umi";
import { queryCurrentUser } from "@/services/global";
import styles from "@/pages/Notebooks/style.less";
var permissionNamespaces = []
const TableList = ({ globalConfig }) => {
    const intl = useIntl();
    const [loading, setLoading] = useState(false);
    const [jobs, setJobs] = useState([]);
    const [total, setTotal] = useState(0);
    const [users, setUsers] = useState({});
    const [selected, setSelected] = useState([]);
    const [namespaces, setNamespaces] = useState([]);

    const pageSizeRef = useRef(20);
    const currentRef = useRef(1);
    const paramsRef = useRef({});
    const fetchIntervalRef = useRef();
    const actionRef = useRef();
    const formRef = useRef();

    const onTableSelectChange = selectedRowKeys => {
        setSelected(selectedRowKeys)
    };

    const selectedRowKeys = selected

    const rowSelection = {
        selectedRowKeys,
        onChange: onTableSelectChange,
    };

    const searchInitialParameters = {
        jobStatus: "All",
        submitDateRange: [moment().subtract(30, "days"), moment()],
        current: 1,
        page_size: 20,
    };

    useEffect(() => {
        fetchUser().then();
        fetchJobs().then();
        const interval = 10 * 1000;
        fetchIntervalRef.current = setInterval(() => {
            fetchJobsSilently();
        }, interval);
        return () => {
            clearInterval(fetchIntervalRef.current);
        };
    }, []);

    const fetchJobs = async () => {
        setLoading(true);
        await fetchJobsSilently();
        setLoading(false);
    };

    const fetchUser = async () => {
        const currentUser = await queryCurrentUser();
        const userInfos = currentUser.data && currentUser.data.loginId ? currentUser.data : {};
        setUsers(userInfos);

        let newNamespaces = [];
        if(userInfos && userInfos.namespaces) {
            for(let idx in userInfos.namespaces) {
                newNamespaces.push(userInfos.namespaces[idx])
            }
        }
        setNamespaces(newNamespaces);
        permissionNamespaces = newNamespaces
    };

    const fetchJobsSilently = async () => {
        let queryParams = { ...paramsRef.current };
        if (!paramsRef.current.submitDateRange) {
            queryParams = {
                ...queryParams,
                ...searchInitialParameters,
            };
        }
        let jobs = await queryJobs({
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
            page_size: pageSizeRef.current,
        });
        if (jobs && jobs.data) {
            let jobList = []
            for (let idx = 0;idx < jobs.data.length;idx++) {
                for(let i = 0;i < permissionNamespaces.length;i++){
                    if(permissionNamespaces[i] === jobs.data[idx].namespace){
                        jobList.push(jobs.data[idx])
                    }
                }
            }

            for (let index = 0; index < jobList.length; index++) {
                jobList[index].key = index + 1
            }
            setJobs(jobList);
            setTotal(jobList.length);
        }
    };

    const showMetrics = (job) => {
        let localStorage = window.localStorage;
        localStorage.setItem('id', job.id)
        localStorage.setItem('namespace', job.namespace)
        localStorage.setItem('name', job.name)
        history.push({
            pathname: `/evaluateJobs/metrics`,
            query: {}
        });
    }

    const onJobDelete = (job) => {
        Modal.confirm({
            title: intl.formatMessage({ id: "dlc-dashboard-delete-job" }),
            icon: <ExclamationCircleOutlined />,
            content: `${intl.formatMessage({
                id: "dlc-dashboard-delete-job-confirm",
            })} ${job.name} ?`,
            onOk: () =>
                deleteJobs(
                    job.namespace,
                    job.name
                ).then(() => {
                    const { current } = actionRef;
                    if (current) {
                        current.reload();
                    }
                    fetchJobs()
                }),
            onCancel() { },
        });
    };

    const onSearchSubmit = (params) => {
        paramsRef.current = params;
        fetchJobs();
    };

    const onTableChange = (pagination) => {
        if (pagination) {
            currentRef.current = pagination.current;
            pageSizeRef.current = pagination.pageSize;
            fetchJobs();
        }
    };

    const Tip = ({ dlc, Click, disabled, IconComponent }) => {
        return (
            <Tooltip >
                {/*title={intl.formatMessage({ id: dlc })}>*/}
                <a onClick={() => Click()} disabled={disabled}>
                    {IconComponent}
                </a>
            </Tooltip>
        );
    };

    const toEvaluateJobCompare = () => {
        let param = []
        if (selected.length < 2) {
            alert("At least more than 2")
            return
        }
        for (let index = 0; index < selected.length; index++) {
            let temp = {
                name: jobs[selected[index] - 1].name,
                namespace: jobs[selected[index] - 1].namespace,
                id:  jobs[selected[index]-1].id
            }
            param.push(temp)
        }
        let localStorage = window.localStorage;
        localStorage.setItem('param', JSON.stringify(param))
        history.push({
            pathname: `/evaluateJobs/compare`,
            query: {}
        });
    }

    let columns = [
        {
            title: "Name",
            dataIndex: "name",
            hideInSearch: true,
            render: (_, record) => {
                return (
                    <a onClick={() => {
                        let localStorage=window.localStorage;
                        // localStorage.setItem('namespace', record.namespace)
                        // localStorage.setItem('name', record.name)
                        localStorage.setItem('id', record.id)
                        history.push({
                            pathname: `/evaluateJobs/metrics`,
                            query: {}
                        });
                    }}>
                        {record.name}
                    </a>
                )
            },
        },
        {
            // title: 'Date Range',
            title: intl.formatMessage({ id: "dlc-dashboard-time-interval" }),
            dataIndex: "submitDateRange",
            valueType: "dateRange",
            initialValue: searchInitialParameters.submitDateRange,
            hideInTable: true,
        },
        {
            // title: 'End Time',
            title: intl.formatMessage({ id: "Model Name" }),
            dataIndex: "modelName",
            //valueType: "date",
            hideInSearch: true,
        },
        {
            // title: 'End Time',
            title: intl.formatMessage({ id: "Model Version" }),
            dataIndex: "modelVersion",
            //valueType: "date",
            hideInSearch: true,
        },
        {
            // title: 'Namespace',
            title: intl.formatMessage({ id: "dlc-dashboard-namespace" }),
            dataIndex: "namespace",
            hideInSearch: true,
        },
        {
            // title: 'Status',
            title: intl.formatMessage({ id: "dlc-dashboard-status" }),
            width: 128,
            dataIndex: "jobStatus",
            initialValue: searchInitialParameters.jobStatus,
            hideInSearch: true,
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
            dataIndex: "ModifyTime",
            //valueType: "date",
            hideInSearch: true,
        },
        {
            title: intl.formatMessage({ id: "dlc-dashboard-operation" }),
            dataIndex: "option",
            valueType: "option",
            render: (_, record) => {
                let isDisabled = true;
                return (
                    <Fragment>

                        <Tip
                            dlc={"dlc-dashboard-delete"}
                            Click={onJobDelete.bind(this, record)}
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

    return (
        <PageHeaderWrapper title={<></>}>
            <ProTable
                toolBarRender={() => [
                    // <Select
                    //     // onChange={toChangeNamespace}
                    //     className={styles.namespaces}
                    //     placeholder={intl.formatMessage({id: 'change namespace'})}
                    // >
                    //     {namespaces.map(data => (
                    //         data &&
                    //         <Select.Option title={data} value={data} key={data}>
                    //             {data}
                    //         </Select.Option>
                    //     ))}
                    // </Select>,
                    <Button type="primary" key="primary" onClick={toEvaluateJobCompare}>
                        {intl.formatMessage({ id: "Metrics Compare" })}
                    </Button>
                ]}
                rowSelection={rowSelection}
                loading={loading}
                dataSource={jobs}
                onSubmit={(params) => onSearchSubmit(params)}
                headerTitle={intl.formatMessage({ id: "dlc-dashboard-job-list" })}
                actionRef={actionRef}
                formRef={formRef}
                rowKey="key"
                columns={[...columns]}
                options={{
                    fullScreen: true,
                    setting: true,
                    reload: () => fetchJobs(),
                }}
                onChange={onTableChange}
                pagination={{ total: total }}
                scroll={{ y: 450 }}
            />
        </PageHeaderWrapper>
    );
}

export default connect(({ global }) => ({
    globalConfig: global.config,
}))(TableList);
