import {
    ExclamationCircleOutlined,
    FundViewOutlined,
    PlusSquareOutlined,
    FormOutlined,
    DeleteOutlined,
    CopyOutlined,
} from "@ant-design/icons";
import {Table, Tag, Space, Modal, message, Tooltip, Select, Button, Drawer} from "antd";
import React, {useRef, useState, useEffect, Fragment, Component} from "react";
import { PageHeaderWrapper } from "@ant-design/pro-layout";
import echarts from 'echarts/lib/echarts'
import 'echarts/lib/component/tooltip';
import 'echarts/lib/component/title';
import 'echarts/lib/chart/line';
import 'echarts/lib/chart/bar';
import 'echarts/lib/component/grid';
import ReactEcharts from 'echarts-for-react';
import {
    getEvaluateJobCompareData,
} from "./service";

import { connect, useIntl, history } from "umi";

var template = {
    xAxis: {
        type: 'category',
        data: [],
    },
    yAxis: {
        type: 'value',
    },
    series: [{
        data: [],
        type: 'bar',
        smooth: true,
    }]
};

class MetricsCharts extends Component {
    state = {
        option: this.props.dataSource,
        xData: this.props.xData
    }
    componentDidMount() {}
    getOption =()=> {
        return {
            xAxis: {
                type: 'category',
                data: this.state.xData,
            },
            yAxis: {
                type: 'value',
            },
            series: [{
                data: this.props.dataSource,
                type: 'bar',
                smooth: true,
            }]
        };
    }
    render() {
        return(
            // <ReactEcharts option={this.getOption()} style={{width: 400, height: 400}}/>
            <ReactEcharts option={this.getOption()} style={{height: 400}}/>
        )
    }
}
const MetricsList = ({ globalConfig }) => {
    const intl = useIntl();
    const [visible, setVisible] = useState(false);
    const [xData, setXData] = useState([]);
    const [dataSource, setDataSource] = useState([]);

    useEffect(() => {
        let localStorage=window.localStorage;
        let param = localStorage.getItem("param")
        fetchDataSource(param).then(r => {
            console.log(r)
        })
    }, []);

    const fetchDataSource = async (param) => {
        let response = await getEvaluateJobCompareData(param);
        let metricsData = response.data.metrics
        let result = []
        setXData(response.data.names)
        for(let metricsKey in metricsData) {
            let temp = {
                key: metricsKey,
                value: metricsData[metricsKey]
            }
            result.push(temp)
        }
        setDataSource(result)
    };

    const newCompareChart = (source) => {
            return (
                <MetricsCharts dataSource={source} xData={xData} />
            )
    }

    const onClose = () => {
        setVisible(false)
    }

    const columns = [
        {
            title: intl.formatMessage({ id: "Metrics" }),
            dataIndex: 'key',
            key: 'key',
        },
        {
            title: intl.formatMessage({ id: "Compare" }),
            dataIndex: 'value',
            key: 'value',
            render: (_, record) => (
                <MetricsCharts dataSource={record.value} xData={xData} />
            )
        },
    ]

    return (
        <PageHeaderWrapper title={<></>}>
            <Table dataSource={dataSource} columns={columns} />
        </PageHeaderWrapper>
    );
};

export default connect(({ global }) => ({
    globalConfig: global.config,
}))(MetricsList);
