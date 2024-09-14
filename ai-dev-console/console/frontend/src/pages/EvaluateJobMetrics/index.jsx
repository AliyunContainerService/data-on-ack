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
import echarts from 'echarts/lib/echarts';
import 'echarts/lib/component/tooltip';
import 'echarts/lib/component/title';
import 'echarts/lib/chart/line';
import 'echarts/lib/chart/bar';
import 'echarts/lib/component/grid';
import {
    getEvaluateJob,
} from "./service";

import { connect, useIntl, history } from "umi";

const template = {
    xAxis: {
        type: 'category',
        data: [],
        boundaryGap: false,
    },
    yAxis: {
        type: 'value',
    },
    series: [{
        data: [],
        type: 'line',
        smooth: true,
    }]
};

class MetricsCharts extends Component {
    state = {
        option: this.props.dataSource
    }
    componentDidMount() {
        var chart = echarts.init(document.getElementById('main'));
        chart.setOption(this.state.option)
    }
    render() {
        return(
            <div id="main" style={{width: 400, height: 400}}/>
        )
    }
}
const MetricsList = ({ globalConfig }) => {
    const intl = useIntl();
    const [visible, setVisible] = useState(false);
    const [data, setData] = useState(template);
    const [dataSource, setDataSource] = useState([]);

    useEffect(() => {
        let localStorage=window.localStorage;
        let id = localStorage.getItem("id")
        fetchDataSource(id)
    }, []);

    const fetchDataSource = async (id) => {
        let response = await getEvaluateJob(id);
        if (response.data.metrics == "") {
            return
        }
        let metrics = eval("(" + response.data.metrics + ")")
        let metricsData = []
        for(let metricsKey in metrics) {
            let temp = {
                key: metricsKey,
                value: metrics[metricsKey]
            }
            if (metricsKey == "ROC"){
                let graphData = template
                graphData.xAxis.data = metrics["ROC"].fpr;
                graphData.series[0].data = metrics["ROC"].tpr;
                setData(graphData)
            }
            metricsData.push(temp)
        }
        setDataSource(metricsData)
    };

    // const showDrawer = () => {
    //     setVisible(true)
    // };
    //
    // const onClose = () => {
    //     setVisible(false)
    // }

    const columns = [
        {
            title: 'Key',
            dataIndex: 'key',
            key: 'key',
        },
        {
            title: 'Value',
            dataIndex: 'value',
            key: 'value',
            render: (_, record) => {
                if (record.key == "ROC") {
                    return(
                        // <Button type="primary" onClick={showDrawer}>
                        //     Show
                        // </Button>
                        <MetricsCharts dataSource={data} />
                    )
                }else {
                    return (
                        <a>{record.value}</a>
                    )
                }

            },
        },
    ]

    return (
        <PageHeaderWrapper title={<></>}>
            <Table dataSource={dataSource} columns={columns} />
            {/*<Drawer*/}
            {/*    title="ROC"*/}
            {/*    closable={false}*/}
            {/*    onClose={onClose}*/}
            {/*    visible={visible}*/}
            {/*    width={600}*/}
            {/*>*/}
            {/*    <MetricsCharts dataSource={data} />*/}
            {/*</Drawer>*/}
        </PageHeaderWrapper>
    );
};

export default connect(({ global }) => ({
    globalConfig: global.config,
}))(MetricsList);
