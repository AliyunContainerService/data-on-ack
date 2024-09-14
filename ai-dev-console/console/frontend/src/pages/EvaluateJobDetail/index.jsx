import {Button, Card, Descriptions, Modal, Steps, Table, Empty,Row,Col, Tabs, message} from "antd";
import {PageHeaderWrapper} from "@ant-design/pro-layout";
import {ExclamationCircleOutlined} from "@ant-design/icons";
import React, {Component, Fragment} from "react";
import PageLoading from "@/components/PageLoading";
import JobStatus from "@/components/JobStatus";
import LogModal from "./LogModal";
import { getJobDetail, getEvents, deleteJobs, cloneInfoJobs, getPodRangeCpuInfoJobs, getPodRangeGpuInfoJobs, getPodRangeMemoryInfoJobs, stopJobs } from "./service.js";
import styles from "./style.less";
import moment from "moment";
import { LazyLog } from "react-lazylog";
import { FormattedMessage,history, useIntl, formatMessage } from 'umi';
import PodCharts from '@/pages/JobDetail/PodCharts';
import {queryCurrentUser} from "@/services/global";
const jobDeleteTitleFormatedText = formatMessage({
    id: 'dlc-dashboard-delete-job'
});
const jobStopTitleFormatedText = formatMessage({
    id: 'dlc-dashboard-stop-job'
});
const jobDeleteContentFormatedText = formatMessage({
    id: 'dlc-dashboard-delete-job-confirm'
});
const jobStopContentFormatedText = formatMessage({
    id: 'dlc-dashboard-stop-job-confirm'
});
const jobModelOkText = formatMessage({
    id: 'dlc-dashboard-ok'
});
const jobModelCancelText = formatMessage({
    id: 'dlc-dashboard-cancel'
});

class EvaluateJobDetail extends Component {
    refreshInterval = null;
    state = {
        detailLoading: true,
        detail: {},
        eventsLoading: true,
        events: [],
        total: 0,
        tabActiveKey: "spec",
        logModalVisible: false,
        currentPod: undefined,
        currentPage: 1,
        currentPageSize: 10,
        resourceConfigKey: 'Worker',
        podChartsValue: [],
        podChartsType: 'CPU',
        podChartsLoading: false,
        users:{},
    };
    onResourceConfigTabChange = (key, type) => {
        this.setState({ [type]: key });
    };
    async componentDidMount() {
        await this.fetchDetail();
        await this.fetchUser();
        // await this.fetchGetPodRangeInfoJobs(this.state.podChartsType);
        const interval = 5 * 1000;
        this.refreshInterval = setInterval(() => {
            this.fetchDetailSilently()
        }, interval);
    }
    componentWillUnmount() {
        clearInterval(this.refreshInterval);
    }

    async fetchDetail() {
        this.setState({
            detailLoading: true
        });
        await this.fetchDetailSilently();
        this.setState({
            detailLoading: false,
        })
    }

    fetchUser = async () => {
        const users = await queryCurrentUser();
        const userInfos = users.data ? users.data : {};
        this.setState({
            users: userInfos
        });
    }

    async fetchGetPodRangeInfoJobs(type) {
        this.setState({
            podChartsLoading: true
        });
        const startTime = this.state.detail?.createTime && this.state.detail?.createTime !== '' ? new Date(this.state.detail?.createTime) : null;
        const endTime = this.state.detail?.endTime && this.state.detail?.endTime !== '' ? new Date(this.state.detail.endTime) : new Date(new Date().getTime());
        let info = {};
        if (type === 'CPU') {
            info = await getPodRangeCpuInfoJobs(this.state.detail?.name, Math.floor(startTime.valueOf()/ 1000), Math.floor(endTime.valueOf()/ 1000), '10m', sessionStorage.getItem("namespace"))
        }else if (type === 'GPU') {
            info = await getPodRangeGpuInfoJobs(this.state.detail?.name, Math.floor(startTime.valueOf()/ 1000), Math.floor(endTime.valueOf()/ 1000), '10m', sessionStorage.getItem("namespace"));
        }else {
            info = await getPodRangeMemoryInfoJobs(this.state.detail?.name, Math.floor(startTime.valueOf()/ 1000), Math.floor(endTime.valueOf()/ 1000), '10m', sessionStorage.getItem("namespace"));
        }
        const infoPod = info && info.data ? info.data : [];
        this.setState({
            podChartsValue: this.handlePodChartsData(infoPod)
        }, () => {
            this.setState({
                podChartsLoading: false
            });
        })
    }

    handlePodChartsData = data => {
        const newData = [];
        data && data.length > 0 && data.map((p) => {
            p['values'].map((v) => {
                newData.push({
                    name: p['metric']['pod_name'],
                    x: v[0],
                    y: Number(v[1]).toFixed(2),
                })
            })
        });
        return newData;
    }

    async fetchDetailSilently() {
        const { match, location } = this.props;
        try{
            let res = await getJobDetail({
                job_id: match.params.id,
                ...location.query,
                current_page: this.state.currentPage,
                page_size: this.state.currentPageSize,
            });
            if(res.code ==200){
                this.setState({
                    detail: res.data ? res.data.jobInfo : {},
                    total: res.data ? res.data.total : 0,
                }, () => {
                    const newResources = this.state.detail && this.state.detail.resources ? eval('('+ this.state.detail?.resources +')') : {};
                    this.setState({
                        resourceConfigKey: JSON.stringify(newResources) !== '{}' ? Object.keys(newResources)[0] : '',
                    });
                });
            }else {
                message.error(JSON.stringify(res.data));
            }
        }catch(error){
            message.error(JSON.stringify(error));
        }
    }
    async fetchEvents() {
        const { match, location } = this.props;
        const { detail } = this.state;

        this.setState({
            eventsLoading: true
        })

        let createTime = moment(detail.createTime).toDate().toISOString();
        let res = await getEvents(detail.namespace, detail.name, detail.id, createTime)
        this.setState({
            eventsLoading: false,
            events: res.data || []
        })
    }

    onTabChange = tabActiveKey => {
        const {} = this.props;
        const { detail } = this.state;
        this.setState({
            tabActiveKey
        });
        if (tabActiveKey === "events") {
            this.fetchEvents()
        }
    };

    onLog = p => {
        this.setState({
            currentPod: p,
            logModalVisible: true
        });
    };

    onLogClose = () => {
        this.setState({
            currentPod: undefined,
            logModalVisible: false
        });
    };

    onPaginationChange = (page, pageSize) => {
        this.setState({
            currentPage: page,
            currentPageSize: pageSize
        }, () => {
            this.fetchDetail()
        })
    }

    action = detail => {
        let isDisabled;
        if (this.state.users.accountId === this.state.users.loginId) {
            isDisabled = true;
        }else {
            isDisabled = detail.jobUserId && detail.jobUserId === this.state.users.loginId;
        }
        return (
            <Fragment>
                <Button type="primary" onClick={() => this.fetchJobCloneInfoSilently(detail)} disabled={!isDisabled} >
                    {<FormattedMessage id="dlc-dashboard-clone" />}
                </Button>
                <Button type="danger" onClick={() => this.onJobDelete(detail)} disabled={!isDisabled}>
                    {<FormattedMessage id="component.delete" />}
                </Button>
            </Fragment>
        );
    };
    //克隆
    async fetchJobCloneInfoSilently(job) {
        const { match, location } = this.props;
        const namespace = location.query.namespace;
        let res = await cloneInfoJobs(namespace, job.name, job.jobType);
        const infoData = res?.data ?? {};
        try{
            if (JSON.parse(infoData || "{}").metadata) {
                sessionStorage.setItem("job", infoData);
                history.push({
                    pathname: '/job-submit',
                });
            }
        }catch (e) {
            console.log(e)
        }
    }

    onJobStop = job => {
        Modal.confirm({
            title: jobStopTitleFormatedText,
            icon: <ExclamationCircleOutlined/>,
            content:  `${jobStopContentFormatedText} ${job.name} ?`,
            okText: jobModelOkText,
            cancelText: jobModelCancelText,
            onOk: () =>
                stopJobs(
                    job.namespace,
                    job.name,
                    job.id,
                    job.jobType
                ).then(() => {
                    this.fetchDetail();
                    this.fetchGetPodRangeInfoJobs(this.state.podChartsType)
                }),
            onCancel() {
            }
        });
    }

    onJobDelete = job => {
        Modal.confirm({
            title: jobDeleteTitleFormatedText,
            icon: <ExclamationCircleOutlined/>,
            content:  `${jobStopContentFormatedText} ${job.name} ?`,
            okText: jobModelOkText,
            cancelText: jobModelCancelText,
            onOk: () =>
                deleteJobs(
                    job.namespace,
                    job.name,
                    job.id,
                    job.jobType,
                    moment(job.submitTime)
                        .utc()
                        .format()
                ).then(() => {
                    history.replace('/jobs')
                }),
            onCancel() {
            }
        });
    };

    goToDatasheets = () => {
        return history.push({
            pathname: '/datasheets',
        });
    }

    description = detail => {
        const jobConfig = eval('('+ detail.jobConfig +')');
        const jobResources = eval('('+ detail.resources +')');
        let descriptions = (
            <div>
                <Descriptions bordered className={styles.headerList} size="small">
                    {/*<Descriptions.Item label="ID">{detail.id}</Descriptions.Item>*/}
                    <Descriptions.Item label={<FormattedMessage id="dlc-dashboard-job-name" />}>
                        {detail.name}
                    </Descriptions.Item>
                    <Descriptions.Item label={<FormattedMessage id="dlc-dashboard-job-type" />} span={2}>
                        {detail.jobType}
                    </Descriptions.Item>
                    <Descriptions.Item label={<FormattedMessage id="dlc-dashboard-creation-time" />}>
                        {detail.createTime}
                    </Descriptions.Item>
                    <Descriptions.Item label={<FormattedMessage id="dlc-dashboard-end-time" />}>{detail.endTime}</Descriptions.Item>
                    <Descriptions.Item label={<FormattedMessage id="dlc-dashboard-execution-time" />}>
                        {detail.durationTime}
                    </Descriptions.Item>
                </Descriptions>

            </div>
        );
        return (descriptions);
    };
    onPodTabChange = (value) => {
        this.setState({
            podChartsType: value,
            podChartsValue: []
        }, () => {this.fetchGetPodRangeInfoJobs(value);})
    }

    render() {
        const {tabActiveKey, detail, detailLoading, events, eventsLoading, total} = this.state;
        if (detailLoading !== false) {
            return <PageLoading/>;
        }
        const title = (
            <span>
          <span style={{paddingRight: 12}}>
            {detail.namespace} / {detail.name}
          </span>
          <JobStatus status={detail.jobStatus}/>
        </span>
        );

        return (
            <PageHeaderWrapper
                onBack={() => history.goBack()}
                title={title}
                // extra={environment && environment !=="eflops" && this.action(detail)}
                className={styles.pageHeader}
                content={ detail && this.description(detail)}
                tabActiveKey={tabActiveKey}
                onTabChange={this.onTabChange}
                tabList={[
                    {
                        key: "spec",
                        tab: <FormattedMessage id="dlc-dashboard-instances" />
                    },
                    {
                        key: "events",
                        tab: <FormattedMessage id="dlc-dashboard-events" />
                    }
                ]}
            >
                <div className={styles.main}>
                    {this.state.tabActiveKey === "spec" && (
                        <Card bordered={false}>
                            <Table
                                size="small"
                                pagination={{
                                    total: total,
                                    current: this.state.currentPage,
                                    pageSize: this.state.currentPageSize,
                                    onChange: this.onPaginationChange
                                }}
                                columns={[
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-name" />,
                                        width: 128,
                                        dataIndex: "name",
                                        key: "name"
                                    },
                                    /*
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-type" />,
                                        dataIndex: "replicaType",
                                        key: "replicaType"
                                    },
                                     */
                                    {
                                        title: "IP",
                                        dataIndex: "containerIp",
                                        key: "containerIp"
                                    },
                                    {
                                        title: "HostIP",
                                        dataIndex: "hostIp",
                                        key: "hostIp"
                                    },
                                    {
                                        title: "GPU",
                                        dataIndex: "gpu",
                                        key: "gpu"
                                    },
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-status" />,
                                        width: 128,
                                        dataIndex: "jobStatus",
                                        key: "jobStatus",
                                        render: (_, r) => <JobStatus status={r.jobStatus}/>
                                    },
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-creation-time" />,
                                        dataIndex: "createTime",
                                        key: "createTime"
                                    },
                                    /*
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-startup-time" />,
                                        dataIndex: "startTime",
                                        key: "startTime"
                                    },
                                     */
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-end-time" />,
                                        dataIndex: "endTime",
                                        key: "endTime"
                                    },
                                    {
                                        title: <FormattedMessage id="dlc-dashboard-operation" />,
                                        dataIndex: "options",
                                        render: (_, r) => (
                                            <>
                                                <a onClick={() => this.onLog(r)}><FormattedMessage id="dlc-dashboard-logs" /></a>
                                            </>
                                        )
                                    }
                                ]}
                                dataSource={detail.specs}
                            />
                        </Card>
                    )}
                    {this.state.tabActiveKey === "events" && (
                        <Card loading={eventsLoading}>
                            {events.length === 0
                                ? <Empty description={<FormattedMessage id="dlc-dashboard-no-events" />}/>
                                : <div style={{minHeight: 256}}>
                                    <LazyLog
                                        extraLines={1}
                                        enableSearch
                                        text={events.join('\n')}
                                        caseInsensitive
                                    />
                                </div>
                            }
                        </Card>
                    )}
                </div>
                {this.state.logModalVisible && (
                    <LogModal
                        pod={this.state.currentPod}
                        job={detail}
                        onCancel={() => this.onLogClose()}
                    />
                )}
            </PageHeaderWrapper>
        );
    }
}

export default EvaluateJobDetail

