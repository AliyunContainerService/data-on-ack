import {
    DownOutlined,
    QuestionCircleTwoTone,
    PlusOutlined,
    MinusOutlined
} from "@ant-design/icons";
import {
    Button,
    Alert,
    Dropdown,
    Menu,
    Select,
    Tabs,
    Radio,
    Card,
    Row,
    Col,
    Form,
    Input,
    InputNumber,
    Switch,
    DatePicker,
    Space
} from "antd";
import React, { useState,  useEffect, Fragment } from "react";
import PropTypes from 'prop-types'
import { connect } from "dva";
import { PageHeaderWrapper } from "@ant-design/pro-layout";
import { getDatasources, submitJob, getCodeSource } from "./service";
import Tooltip from "antd/es/tooltip";
import FooterToolbar from "../../components/FooterToolbar";
import { history, useIntl, getLocale, useLocation } from 'umi';
import {queryCurrentUser} from "@/services/global";
import * as ComForm from "../../components/Form";
import styles from "./style.less";

var path = require("path");
const JobCreate = ({ globalConfig }) => {
    const defaultWorkingDir = '/root/';
    const defaultRelativeCodePath = "code/";
    let tfCPUImages = globalConfig["default-tf-cpu-images"] || [];
    let tfGPUImages = globalConfig["default-tf-gpu-images"] || [];
    const tfJobImages = tfCPUImages.concat(tfGPUImages);
    const pyTorchImages = globalConfig["default-pytorch-gpu-images"] || [];
    const intl = useIntl();
    const [submitLoading, setSubmitLoading] = useState(false);
    const [activeTabKey, setActiveTabKey] = useState("Worker");
    const [dataSource, setDataSource] = useState([]);
    const [codeSource, setCodeSource] = useState([]);
    const [namespaces, setNamespaces] = useState([]);

    const [form] = Form.useForm();
    const [cloneInfo, setCloneInfo] = useState(undefined);
    const [usersInfo, setUsersInfo] = useState({});
    const [showCron, setShowCron] = useState(true);

    const [selectedType, setSelectedType] = useState('GPUExclusive');
    
    const [gpuResource, setgpuResource] = useState(0);

    if (sessionStorage.getItem("job")) {
        setCloneInfo(JSON.parse(sessionStorage.getItem("job")));
        sessionStorage.removeItem('job');
    }

    const formInitialTF = {
        name: "",
        kind: "TFJob",
        command: "",
        shell: "sh",
        logDir: "",
        tensorboard: {
            enabled: false,
        },
        cron: {
            enabled: false,
            schedule: "",
            concurrencyPolicy: "",
            deadline: "",
            historyLimit: 10,
        },
        tasks: [
            {
                role: "Worker",
                replicas: 1,
                resource: {
                    gpu: 0,
                    cpu: 4,
                    memory: 8
                },
                image: tfJobImages[0]
            }
        ],
        dataSource: null,
        codeSource: null,
        codeSourceBranch: "",
        workingDir: defaultWorkingDir,
        imgPullSecrets: null,
        namespaces: null,
        annotations: null,
        labels: null,
        nodeSelectors: null,
        ttlSecondsAfterFinished: null,
    };

    const formInitialPyTorch = {
        ...formInitialTF,
        kind: "PyTorchJob",
        tasks: [
            {
                role: "Worker",
                replicas: 1,
                resource: {
                    gpu: 1,
                    cpu: 4,
                    memory: 8
                },
                image: pyTorchImages[0]
            }
        ],
    }

    const location = useLocation();
    const queryParams = new URLSearchParams(location.search);
    const sourceParam = queryParams.get("source");
    if (sourceParam == "clone") {
        const cloneJobObj = JSON.parse(localStorage.getItem('cloneJob'));
        formInitialTF.kind = cloneJobObj.jobType
        formInitialTF.name = cloneJobObj.name
        formInitialTF.namespaces = cloneJobObj.namespace
        formInitialTF.command = JSON.parse(cloneJobObj.jobConfig).commands.join(' ');
    }

    useEffect(() => {
        fetchSource();
        fetchUser();
    }, []);

    const fetchSource = async () => {
        const gitSource = await getCodeSource();

        const newGitSource = [];
        if (gitSource && gitSource.data) {
            for (const key in gitSource.data) {
                newGitSource.push(gitSource.data[key]);
            }
        }

        setCodeSource(newGitSource);
    };

    const fetchUser = async () => {
        const currentUser = await queryCurrentUser();
        const userInfos = currentUser.data && currentUser.data.loginId ? currentUser.data : {};
        setUsersInfo(userInfos);

        const newNamespaces = [];
        if(userInfos && userInfos.namespaces) {
            for(let idx in userInfos.namespaces) {
                newNamespaces.push(userInfos.namespaces[idx])
            }
        }
        setNamespaces(newNamespaces);
    }

    const gpuResourceChange = (e) => {
        setgpuResource(e)
    }

    const onFormSubmit = async form => {
        const newFormKind = ["TFJob", "TFJobDistributed"].includes(form.kind) ? 'TFJob' : 'PyTorchJob';

        const data = {
            name: form.name,
            kind: newFormKind,
            annotations: {},
            command: [form.command],
            shell: form.shell,
            workingDir: defaultWorkingDir,
            enableTensorboard: form.tensorboard.enabled,
            logDir: form.logDir,
            tensorboardHost: window.location.hostname,
            imagePullSecrets: [],
            volumes: {},
        }

        if (showCron && form.cron) {
            data.enableCron = form.cron.enabled;
            data.schedule = form.cron.schedule;
            data.concurrencyPolicy = form.cron.concurrencyPolicy;
            data.deadline = form.cron.deadline;
            data.historyLimit = form.cron.historyLimit;
        }

        if (form.labels) {
            data.labels = form.labels.filter(d => d.key).reduce((p,c) => {p[c.key] = c.value; return p}, {})
        }

        if (form.annotations) {
            data.annotations = form.annotations.filter(d => d.key).reduce((p,c) => {p[c.key] = c.value; return p}, {})
        }

        if (form.nodeSelectors) {
            data.nodeSelectors = form.nodeSelectors.reduce((p,c) => {p[c.key] = c.value; return p}, {})
        }

        if (form.labels) {
            data.labels = form.labels.filter(d => d.key).reduce((p,c) => {p[c.key] = c.value; return p}, {})
        }

        if (form.devices) {
            data.devices = form.devices.filter(d => d.key).reduce((p,c) => {p[c.key] = c.value; return p}, {})
        }

        if (form.tolerations) {
            data.tolerates = form.tolerations.filter(d => d.key).reduce((p,c) => {
                p[c.key] = {};
                p[c.key].operator = c.operator;
                p[c.key].value = c.value;
                p[c.key].effect = c.effect;
                return p
            }, {})
        }

        if (form.ttlSecondsAfterFinished !== null && form.ttlSecondsAfterFinished !== undefined) {
            data.ttlSecondsAfterFinished = form.ttlSecondsAfterFinished * 3600
        }

        let validPVCNamespace = true;
        const dataSourceNameObj ={};
        dataSource.forEach(({name,...other})=>{
            dataSourceNameObj[name] = {...other, name};
        })
        if(form.dataSource && form.dataSource.length >0){
            form.dataSource.forEach(({dataSource})=>{
                if(dataSourceNameObj[dataSource]){
                    let {name, pvc_name, local_path, namespace} = dataSourceNameObj[dataSource];
                    data.volumes[pvc_name] = local_path
                    if(namespace != data.namespace){
                        validPVCNamespace=false;
                    }
                }
            })
        }

        const buildLocalPath = function(path) {
            let newPath = path;
            if(!path.endsWith("/")) {
                newPath = path + "/";
            }
            return newPath;
        }

        const findCodeSource = codeSource.filter((c) => c.name === form.codeSource)[0];
        if (findCodeSource && findCodeSource.type === "git") {
            let gitRepoName = path.basename(findCodeSource['code_path'], path.extname(findCodeSource['code_path']));
            data.codeType = findCodeSource.type
            data.codeSource = findCodeSource['code_path']
            data.codeBranch = form.codeSourceBranch || findCodeSource['default_branch']
            // data.codeDestPath = defaultWorkingDir + defaultRelativeCodePath + gitRepoName
            data.codeDestPath = buildLocalPath(findCodeSource['local_path']) + defaultRelativeCodePath + gitRepoName
            data.workingDir = buildLocalPath(findCodeSource['local_path'])

            if(findCodeSource['git_username'] != "" && findCodeSource["git_password"] != "") {
                data.codeUser = findCodeSource['git_username']
                data.codePassword = findCodeSource['git_password']
            }
        }

        const replicaSpecs = (task)=>{
            data[task.role.toLowerCase()+"Count"]=task.replicas
            data[task.role.toLowerCase()+"Image"]=task.image
            data[task.role.toLowerCase()+"CPU"]=task.resource.cpu+""
            data[task.role.toLowerCase()+"Memory"]=task.resource.memory+"Gi"
            data[task.role.toLowerCase()+"GPU"]=task.resource.gpu
            if (gpuResource > 0 && selectedType == 'GPUExclusive') {
                data[task.role.toLowerCase()+"GPU"] = gpuResource
            }
        };

        form.tasks.forEach(task => {
            replicaSpecs(task);
        });

        if (form.imgPullSecrets !== undefined && form.imgPullSecrets !== null && form.imgPullSecrets.length !== 0) {
            data.imagePullSecrets = [form.imgPullSecrets]
        }

        if (form.namespaces !== undefined && form.namespaces != null && form.namespaces.length !== 0) {
            data.namespace = form.namespaces[0]
        }

        if (gpuResource > 0 && selectedType == 'GPUShare') {
            if (typeof data.devices == 'undefined') {
                data.devices = {}
            }
            gpuMemResourceGib = Math.round(gpuResource * 1000 * 1000 * 1000 / (1024 * 1024 * 1024))
            data.devices['aliyun.com/gpu-mem'] = gpuMemResourceGib
        }

        // data.annotations['kubeflow.org/tenancy'] = JSON.stringify({
        //     tenant: "",
        //     user: usersInfo.loginId ? usersInfo.loginId : '',
        // });
        data.command=[form.command]
        console.log(data)

        try {
            setSubmitLoading(true);
            let ret = await submitJob(data, newFormKind);
            if (ret.code === "200") {
                if(data.enableCron) {
                    history.push("/crons");
                } else {
                    history.push("/jobs");
                }
            }
        } finally {
            setSubmitLoading(false);
        }
    };

    const onTaskTabEdit = (targetKey, action, fieldOps) => {
        if (action === "remove") {
            const tasks = form.getFieldValue("tasks");
            const removeIndex = tasks.map(t => t.role).indexOf(targetKey);
            setActiveTabKey(tasks[removeIndex - 1].role);
            fieldOps.remove(removeIndex);
        }
    };

    const onTabChange = key => {
        setActiveTabKey(key);
    };
    const onTaskAdd = (e, fieldOps) => {
        form.validateFields([["tasks"]]).then(() => {
            const role = e.key;
            fieldOps.add({
                role: role,
                command: "",
                replicas: 1,
                resource: {
                    gpu: 0,
                    cpu: 4,
                    memory: 8
                },
                image: tfJobImages[0]
            });
            setActiveTabKey(role);
        });
    };

    const tasksHasRole = role => {
        const tasks = form.getFieldValue("tasks");
        if(tasks!==undefined){
            return tasks.some(t => t.role === role);
        }
    };

    const changeTaskType = value => {
        setActiveTabKey("Worker");
        const currentFormInitial = form.getFieldsValue() || {};
        currentFormInitial.kind = value;
        if(["TFJob", "TFJobDistributed"].includes(value)){// 选择TF单击和TF发布
            currentFormInitial.tasks = formInitialTF.tasks;
            form.setFieldsValue(currentFormInitial);
            setShowCron(true);
        }else{
            currentFormInitial.tasks = formInitialPyTorch.tasks;
            form.setFieldsValue(currentFormInitial);
            setShowCron(false);
        }
    };

    const formItemLayout = {
        labelCol: { span: getLocale() === 'zh-CN' ? 4 : 8 },
        wrapperCol: { span: getLocale() === 'zh-CN' ? 20 : 16 }
    };

    const handleGitUrl = (url) => {
        if (url && url !== '') {
            const index = url.lastIndexOf("\/");
            const subUrl = url.substring(index + 1,url.length);
            if (subUrl.indexOf(".git") != -1) {
                return subUrl.split('.')[0];
            }
            return url.substring(index + 1,url.length);
        }
        return ''
    }

    const addTaskType = (fieldOps)=> (
        <Dropdown overlay={
            <Menu onClick={e => onTaskAdd(e, fieldOps)}>
                <Menu.Item
                    key="Worker"
                    disabled={tasksHasRole("Worker")}>
                    Worker
                </Menu.Item>
                <Menu.Item key="PS" disabled={tasksHasRole("PS")}>
                    PS
                </Menu.Item>
                <Menu.Item
                    key="Chief"
                    disabled={tasksHasRole("Chief")}>
                    Chief
                </Menu.Item>
                <Menu.Item
                    key="Evaluator"
                    disabled={tasksHasRole("Evaluator")}>
                    Evaluator
                </Menu.Item>
            </Menu>}>
            <Button type="primary">
                {intl.formatMessage({id: 'dlc-dashboard-add-task-type'})} <DownOutlined />
            </Button>
        </Dropdown>
    );

    const gitSourceChange = (v) => {
        const currentFormInitial = form.getFieldsValue() || {};
        const currentDefaultBranch = codeSource.filter((c) => c.name === v)[0] || {};
        currentFormInitial.codeSourceBranch = v ? currentDefaultBranch.default_branch : "";
        form.setFieldsValue(currentFormInitial);
    }

    const namespaceChange = (v) => {
        const currentFormInitial = form.getFieldsValue() || {};
        currentFormInitial.namespaces = [v];
        form.setFieldsValue(currentFormInitial);
        fetchDataSource(v).then()
    }

    const shellChange = (v) => {
        const currentFormInitial = form.getFieldsValue() || {};
        currentFormInitial.shell = v.target.value;
        form.setFieldsValue(currentFormInitial);
    }

    const fetchDataSource = async (namespaceValue) => {
        let dataSource = await getDatasources();
        let ns = namespaceValue;
        let newDataSource = [];
        if (dataSource && dataSource.data) {
            for (let key in dataSource.data) {
                if (dataSource.data[key].namespace === ns) {
                    newDataSource.push(dataSource.data[key]);
                }
            }
        }
        setDataSource(newDataSource)
    }

    return (
        <PageHeaderWrapper title={<></>}>
            <Form
                initialValues={formInitialTF}
                form={form}
                {...formItemLayout}
                onFinish={onFormSubmit}
                labelAlign="left">
                <Row gutter={[24, 24]}>
                    <Col span={13}>
                        <Card style={{ marginBottom: 12 }} title={intl.formatMessage({id: 'dlc-dashboard-basic-info'})}>
                            <Form.Item
                                name="name"
                                label={intl.formatMessage({id: 'dlc-dashboard-job-name'})}
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'dlc-dashboard-job-name-required'})},
                                    {
                                        pattern: /^[a-z][-a-z0-9]{0,28}[a-z0-9]$/,
                                        message: intl.formatMessage({id: 'dlc-dashboard-job-name-required-rules'})
                                    }
                                ]}
                            >
                                <Input />
                            </Form.Item>
                            <ComForm.FormSel
                                form ={form}
                                {...{name:"kind", label:intl.formatMessage({id: 'dlc-dashboard-job-type'}), rules:[{ required: true }]}}
                                listOption={[
                                    {label:`TF ${intl.formatMessage({id: 'dlc-dashboard-stand-alone'})}`, value:"TFJob"},
                                    {label:`TF ${intl.formatMessage({id: 'dlc-dashboard-distributed'})}`, value:"TFJobDistributed"},
                                    {label:`Pytorch ${intl.formatMessage({id: 'dlc-dashboard-stand-alone'})}`, value:"PyTorchJob"},
                                    {label:`Pytorch ${intl.formatMessage({id: 'dlc-dashboard-distributed'})}`, value:"PyTorchJobDistributed"},
                                ]}
                                onChange={changeTaskType}
                            />
                            <Form.Item
                                shouldUpdate
                                noStyle>
                                {() =>(
                                    <div>
                                        <div className={getLocale() === 'zh-CN' ? styles.gitSourceContainer : styles.gitSourceContainerEn}>
                                            <Form.Item
                                                label= {intl.formatMessage({id: 'dlc-dashboard-job-namespace'})}
                                                name="namespaces"
                                                rules={[{ required: true, }]}
                                            >
                                                <Select
                                                    onChange={namespaceChange}
                                                    allowClear={true}
                                                >
                                                    {namespaces.map(data => (
                                                        data &&
                                                        <Select.Option title={data} value={data} key={data}>
                                                            {data}
                                                        </Select.Option>
                                                    ))}
                                                </Select>
                                            </Form.Item>
                                        </div>
                                    </div>
                                )}
                            </Form.Item>
                            <ComForm.FromAddDropDown
                                form={form}
                                fieldCode={"dataSource"}
                                fieldKey={"dataSource"}
                                options={dataSource}
                                label={intl.formatMessage({id: 'dlc-dashboard-data-config'})}
                                colStyle={{ labelCol:{ span: getLocale() === 'zh-CN' ? 4 : 8  },
                                    wrapperCol:{ span: getLocale() === 'zh-CN' ? 24 : 16  }
                                }}
                                messageLable={intl.formatMessage({id: 'dlc-dashboard-pvc-name'})}
                            />

                            <Form.Item
                                shouldUpdate
                                noStyle>
                                {() =>(
                                    <div>
                                        <div className={getLocale() === 'zh-CN' ? styles.gitSourceContainer : styles.gitSourceContainerEn}>
                                            <Form.Item
                                                label= {intl.formatMessage({id: 'dlc-dashboard-code-config'})}
                                                name="codeSource"
                                            >
                                                <Select
                                                    onChange={gitSourceChange}
                                                    allowClear={true}
                                                >
                                                    {codeSource.map(data => (
                                                        data &&
                                                        <Select.Option title={data.name} value={data.name} key={data.name}>
                                                            {data.name}
                                                        </Select.Option>
                                                    ))}
                                                </Select>
                                            </Form.Item>
                                        </div>
                                        {![null, "", undefined].includes(form.getFieldValue("codeSource")) &&
                                        <Row gutter={[24, 24]}>
                                            <Col span={getLocale() === 'zh-CN' ? 20 : 16} offset={getLocale() === 'zh-CN' ? 4 : 8}>
                                                <Alert
                                                    type="info"
                                                    showIcon
                                                    message={
                                                        <span>
                                                            {intl.formatMessage({id: 'dlc-dashboard-git-repository'})}
                                                            {': '}
                                                            {
                                                                codeSource.length > 0 &&
                                                                codeSource.some((v) => v.name === form.getFieldValue("codeSource")) &&
                                                                codeSource.filter((v) => v.name === form.getFieldValue("codeSource"))[0]['code_path']
                                                            }
                                                            <br/>
                                                            {intl.formatMessage({id: 'dlc-dashboard-code-local-directory'})}
                                                            {': '}
                                                            {
                                                                codeSource.length > 0 &&
                                                                codeSource.some((v) => v.name === form.getFieldValue("codeSource")) &&
                                                                codeSource.filter((v) => v.name === form.getFieldValue("codeSource"))[0]['local_path'] +
                                                                defaultRelativeCodePath +
                                                                handleGitUrl(codeSource.filter((v) => v.name === form.getFieldValue("codeSource"))[0]['code_path'])
                                                            }
                                                        </span>
                                                    }
                                                />
                                            </Col>
                                        </Row>}
                                        {![null, "", undefined].includes(form.getFieldValue("codeSource")) &&
                                        <React.Fragment>
                                            <Form.Item
                                                label={intl.formatMessage({id: 'dlc-dashboard-code-branch'})}
                                                name={"codeSourceBranch"}
                                                labelCol={{ span: getLocale() === 'zh-CN' ? 3 : 4 , offset: getLocale() === 'zh-CN' ? 4 : 8 }}
                                                wrapperCol={{ span: getLocale() === 'zh-CN' ? 17 : 12  }}
                                            >
                                                <Input placeholder={''}/>
                                            </Form.Item>
                                        </React.Fragment>}
                                    </div>
                                )}
                            </Form.Item>

                            <Form.Item
                                name="imgPullSecrets"
                                label={intl.formatMessage({id: 'dlc-image-pull-secrets'})}
                                rules={[
                                    { required: false},
                                    {
                                        pattern: /^[a-z][-a-z0-9]{0,28}[a-z0-9]$/,
                                        message: intl.formatMessage({id: 'dlc-dashboard-job-name-required-rules'})
                                    }
                                ]}
                            >
                                <Input />
                            </Form.Item >

                            <Form.Item
                                rules={[
                                    { required: false }
                                ]}
                                name="logDir"
                                label={(
                                    <Tooltip title={intl.formatMessage({id: 'dlc-dashboard-training-output-dir-prompt'})} >
                                        {intl.formatMessage({id: 'dlc-dashboard-training-output-dir'})} <QuestionCircleTwoTone twoToneColor="#faad14" />
                                    </Tooltip>
                                )}>
                                <Input placeholder={'/training_logs'} />
                            </Form.Item>

                            <Form.Item
                                label="Shell"
                                name={["shell"]}
                            >
                                <Radio.Group onChange={shellChange}>
                                    <Radio value="sh">sh</Radio>
                                    <Radio value="bash">bash</Radio>
                                </Radio.Group>
                            </Form.Item>

                            <Form.Item
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'dlc-dashboard-execute-command-required'})}
                                ]}
                                name="command"
                                label={intl.formatMessage({id: 'dlc-dashboard-execute-command'})}>
                                <Input.TextArea  placeholder={''}/>
                            </Form.Item>

                            <Form.Item
                                shouldUpdate
                                noStyle
                            >
                                {() =>
                                    (<Form.Item label="Tensorboard">
                                            <Form.Item
                                                name={["tensorboard", "enabled"]}
                                                valuePropName="checked"
                                            >
                                                <Switch />
                                            </Form.Item>
                                        </Form.Item>
                                    )}
                            </Form.Item>

                            {
                                showCron === true && <Form.Item
                                    shouldUpdate
                                    noStyle
                                >
                                    {() =>
                                        (<Form.Item label={intl.formatMessage({id: 'dlc-dashboard-cron'})}>
                                                <Form.Item
                                                    name={["cron", "enabled"]}
                                                    valuePropName="checked"
                                                >
                                                    <Switch />
                                                </Form.Item>
                                                {form.getFieldValue(["cron", "enabled"]) === true &&
                                                <React.Fragment>
                                                    <Form.Item
                                                        name={["cron", "schedule"]}
                                                        label={(
                                                            <Tooltip title={intl.formatMessage({id: 'dlc-dashboard-cron-schedule-prompt'})} >
                                                                {intl.formatMessage({id: 'dlc-dashboard-cron-schedule'})} <QuestionCircleTwoTone twoToneColor="#faad14" />
                                                            </Tooltip>
                                                        )}
                                                        rules={[
                                                            { required: true, message: intl.formatMessage({id: 'dlc-dashboard-cron-schedule-rules'})}
                                                        ]}
                                                        labelCol={{ span: 6 }}
                                                        wrapperCol={{ span: 18 }}
                                                    >
                                                        <Input placeholder={'*/5 * * * *'}/>
                                                    </Form.Item>
                                                    <Form.Item
                                                        name={["cron", "concurrencyPolicy"]}
                                                        label={intl.formatMessage({id: 'dlc-dashboard-cron-concurrency-policy'})}
                                                        labelCol={{ span: 6 }}
                                                        wrapperCol={{ span: 18 }}
                                                        rules={[{ required: true }]}
                                                    >
                                                        <Select>
                                                            <Select.Option value={"Allow"}>Allow</Select.Option>
                                                            <Select.Option value={"Forbid"}>Forbid</Select.Option>
                                                            <Select.Option value={"Replace"}>Replace</Select.Option>
                                                        </Select>
                                                    </Form.Item>
                                                    <Form.Item
                                                        name={["cron", "historyLimit"]}
                                                        label={intl.formatMessage({id: 'dlc-dashboard-cron-history-limit'})}
                                                        labelCol={{ span: 6 }}
                                                        wrapperCol={{ span: 18 }}
                                                    >
                                                        <InputNumber min={1} max={100} defaultValue={10} />
                                                    </Form.Item>
                                                    <Form.Item
                                                        name={["cron", "deadline"]}
                                                        label= {intl.formatMessage({id: 'dlc-dashboard-cron-deadline'})}
                                                        labelCol={{ span: 6 }}
                                                        wrapperCol={{ span: 18 }}
                                                    >
                                                        <DatePicker showTime />
                                                    </Form.Item>
                                                </React.Fragment>}
                                            </Form.Item>
                                        )}
                                </Form.Item>
                            }

                            <Form.Item
                                rules={[
                                    { required: false }
                                ]}
                                name="ttlSecondsAfterFinished"
                                label={(
                                    <Tooltip title={intl.formatMessage({id: 'dlc-dashboard-training-ttl-prompt'})} >
                                        {intl.formatMessage({id: 'dlc-dashboard-training-ttl'})} <QuestionCircleTwoTone twoToneColor="#faad14" />
                                    </Tooltip>
                                )}>
                                <InputNumber
                                    min={1}
                                    max={87600}
                                    precision={0}
                                    style={{ width: "100%" }}
                                />
                            </Form.Item>

                        </Card>
                    </Col>
                    <Col span={11}>
                        <Card title={intl.formatMessage({id: 'dlc-dashboard-resource-info'})} style={{ marginBottom: 12 }}>
                            <Form.List name="tasks">
                                {(fields, fieldOps) => (
                                    <Tabs
                                        type="editable-card"
                                        hideAdd
                                        activeKey={activeTabKey}
                                        onEdit={(targetKey, action) =>
                                            onTaskTabEdit(targetKey, action, fieldOps)
                                        }
                                        onChange={activeKey => onTabChange(activeKey)}
                                        tabBarExtraContent={['TFJobDistributed'].includes(form.getFieldValue('kind')) ? addTaskType(fieldOps) : null }>
                                        {fields.map((field, idx) => (
                                            // <span>{field.name}/{field.fieldKey}</span>
                                            <Tabs.TabPane
                                                tab={form.getFieldValue("tasks")[idx].role}
                                                key={form.getFieldValue("tasks")[idx].role}
                                                closable={
                                                    form.getFieldValue("tasks")[idx].role !== "Worker"}>
                                                <Form.Item
                                                    name={[field.name, "replicas"]}
                                                    label={intl.formatMessage({id: 'dlc-dashboard-instances-num'})}
                                                    fieldKey={[field.fieldKey, "replicas"]}
                                                    rules={[{ required: true, message:intl.formatMessage({id: 'dlc-dashboard-instances-num-required'}) }]}>
                                                    <InputNumber
                                                        min={1}
                                                        step={1}
                                                        precision={0}
                                                        style={{ width: "100%" }}
                                                        disabled={
                                                            form.getFieldValue("tasks")[idx].role === "Chief" ||
                                                            ["TFJob", "PyTorchJob"].includes(form.getFieldValue("kind"))
                                                        }/>
                                                </Form.Item>
                                                <Form.Item
                                                    name={[field.name, "image"]}
                                                    label={intl.formatMessage({id: 'dlc-dashboard-image'})}
                                                    fieldKey={[field.fieldKey, "image"]}
                                                    rules={[
                                                        { required: true, message: intl.formatMessage({id: 'dlc-dashboard-image-required'})}
                                                    ]}>
                                                    <Input />
                                                </Form.Item>
                                                <Form.Item
                                                    name={[field.name, "resource", "cpu"]}
                                                    label={intl.formatMessage({id: 'dlc-dashboard-cpu'})}
                                                    fieldKey={[field.fieldKey, "resource", "cpu"]}>
                                                    <InputNumber
                                                        min={1}
                                                        max={96}
                                                        step={1}
                                                        precision={0}
                                                        style={{ width: "100%" }}/>
                                                </Form.Item>
                                                <Form.Item
                                                    name={[field.name, "resource", "memory"]}
                                                    label={intl.formatMessage({id: 'dlc-dashboard-memory'})}
                                                    fieldKey={[field.fieldKey, "resource", "memory"]}>
                                                    <Select>
                                                        <Select.Option value={1}>1GB</Select.Option>
                                                        <Select.Option value={2}>2GB</Select.Option>
                                                        <Select.Option value={4}>4GB</Select.Option>
                                                        <Select.Option value={8}>8GB</Select.Option>
                                                        <Select.Option value={16}>16GB</Select.Option>
                                                        <Select.Option value={32}>32GB</Select.Option>
                                                        <Select.Option value={64}>64GB</Select.Option>
                                                        <Select.Option value={128}>128GB</Select.Option>
                                                        <Select.Option value={256}>256GB</Select.Option>
                                                    </Select>
                                                </Form.Item>
                                                <Form.Item
                                                    name={[field.name, "resource", "gpu"]}
                                                    label={intl.formatMessage({id: 'dlc-dashboard-gpu'})}
                                                    fieldKey={[field.fieldKey, "resource", "gpu"]}
                                                    dependencies={["tasks"]}>
                                                    <InputNumber
                                                        min={0}
                                                        max={8}
                                                        step={1}
                                                        precision={0}
                                                        style={{ width: "100%" }}
                                                    />
                                                </Form.Item>
                                                <Form.Item label="Type">
                                                    <Radio.Group 
                                                        value={selectedType}
                                                        onChange={(e) => {
                                                            setSelectedType(e.target.value);
                                                            console.log(e)
                                                        }}
                                                    >
                                                        <Radio value="GPUExclusive">Exclusive GPU</Radio>
                                                        <Radio value="GPUShare">GPU Memory(GB)</Radio>
                                                    </Radio.Group>
                                                </Form.Item>
                                                <Form.Item
                                                    label="GPU Resource Number">
                                                    <InputNumber
                                                        value={gpuResource}
                                                        onChange={gpuResourceChange}
                                                        min={0}
                                                        max={96}
                                                        step={1}
                                                        precision={0}
                                                        style={{ width: "100%" }}/>
                                                </Form.Item>
                                            </Tabs.TabPane>
                                        ))}
                                    </Tabs>
                                )}
                            </Form.List>
                        </Card>
                        <Card title={intl.formatMessage({id: 'dlc-dashboard-training-advance-config'})} style={{ marginBottom: 12 }}>
                            <Form.Item label="Device">
                                <Form.List name="devices">
                                    {(fields, {add, remove}) => (
                                        <Fragment>
                                            {fields.map(({key, name, fieldKey, ...restField}) => (
                                                <Space key={key} align="baseline" size="small">
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'key']}
                                                        fieldKey={[fieldKey, 'key']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'value']}
                                                        fieldKey={[fieldKey, 'value']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Button type="ghost" shape="circle" size="small" onClick={() => remove(name)} icon={<MinusOutlined />} />
                                                </Space>
                                            ))}
                                            <Form.Item>
                                                <Button type="ghost" shape="circle" size="small" onClick={() => add()} icon={<PlusOutlined />} />
                                            </Form.Item>
                                        </Fragment>
                                    )}
                                </Form.List>
                            </Form.Item>
                            <Form.Item label="Label">
                                <Form.List name="labels">
                                    {(fields, {add, remove}) => (
                                        <Fragment>
                                            {fields.map(({key, name, fieldKey, ...restField}) => (
                                                <Space key={key} align="baseline" size="small">
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'key']}
                                                        fieldKey={[fieldKey, 'key']}
                                                        rules={[{pattern: /^(([A-Za-z0-9][-A-Za-z0-9_.]*[/A-Za-z0-9_.]*)+[A-Za-z0-9])?$/, message: intl.formatMessage({id:'dlc-dashboard-job-label-key-valid'})}]}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'value']}
                                                        fieldKey={[fieldKey, 'value']}
                                                        rules={[{pattern: /^(([A-Za-z0-9][-A-Za-z0-9_.]*[/A-Za-z0-9_.]*)?[A-Za-z0-9])?$/, message: intl.formatMessage({id:'dlc-dashboard-job-label-value-valid'})}]}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Button type="ghost" shape="circle" size="small" onClick={() => remove(name)} icon={<MinusOutlined />} />
                                                </Space>
                                            ))}
                                            <Form.Item>
                                                <Button type="ghost" shape="circle" size="small" onClick={() => add()} icon={<PlusOutlined />} />
                                            </Form.Item>
                                        </Fragment>
                                    )}
                                </Form.List>
                            </Form.Item>
                            <Form.Item label="Annotation">
                                <Form.List name="annotations">
                                    {(fields, {add, remove}) => (
                                        <Fragment>
                                            {fields.map(({key, name, fieldKey, ...restField}) => (
                                                <Space key={key} align="baseline" size="small">
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'key']}
                                                        fieldKey={[fieldKey, 'key']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'value']}
                                                        fieldKey={[fieldKey, 'value']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Button type="ghost" shape="circle" size="small" onClick={() => remove(name)} icon={<MinusOutlined />} />
                                                </Space>
                                            ))}
                                            <Form.Item>
                                                <Button type="ghost" shape="circle" size="small" onClick={() => add()} icon={<PlusOutlined />} />
                                            </Form.Item>
                                        </Fragment>
                                    )}
                                </Form.List>
                            </Form.Item>
                            <Form.Item label="NodeSelector">
                                <Form.List name="nodeSelectors">
                                    {(fields, {add, remove}) => (
                                        <Fragment>
                                            {fields.map(({key, name, fieldKey, ...restField}) => (
                                                <Space key={key} align="baseline" size="small">
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'key']}
                                                        fieldKey={[fieldKey, 'key']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        name={[name, 'value']}
                                                        fieldKey={[fieldKey, 'value']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Button type="ghost" shape="circle" size="small" onClick={() => remove(name)} icon={<MinusOutlined />} />
                                                </Space>
                                            ))}
                                            <Form.Item>
                                                <Button type="ghost" shape="circle" size="small" onClick={() => add()} icon={<PlusOutlined />} />
                                            </Form.Item>
                                        </Fragment>
                                    )}
                                </Form.List>
                            </Form.Item>
                            <Form.Item label="Toleration">
                                <Form.List name="tolerations">
                                    {(fields, {add, remove}) => (
                                        <Fragment>
                                            {fields.map(({key, name, fieldKey, ...restField}) => (
                                                <Space key={key} align="baseline" size="small">
                                                    <Form.Item
                                                        {...restField}
                                                        label="Key"
                                                        name={[name, 'key']}
                                                        fieldKey={[fieldKey, 'key']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        label="Operator"
                                                        name={[name, 'operator']}
                                                        fieldKey={[fieldKey, 'operator']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        label="Value"
                                                        name={[name, 'value']}
                                                        fieldKey={[fieldKey, 'value']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Form.Item
                                                        {...restField}
                                                        label="Effect"
                                                        name={[name, 'effect']}
                                                        fieldKey={[fieldKey, 'effect']}
                                                    >
                                                        <Input />
                                                    </Form.Item>
                                                    <Button type="ghost" shape="circle" size="small" onClick={() => remove(name)} icon={<MinusOutlined />} />
                                                </Space>
                                            ))}
                                            <Form.Item>
                                                <Button type="ghost" shape="circle" size="small" onClick={() => add()} icon={<PlusOutlined />} />
                                            </Form.Item>
                                        </Fragment>
                                    )}
                                </Form.List>
                            </Form.Item>
                        </Card>
                    </Col>
                </Row>
                <FooterToolbar>
                    <Button type="primary" htmlType="submit">
                        {intl.formatMessage({id: 'dlc-dashboard-submit-job'})}
                    </Button>
                </FooterToolbar>
            </Form>
        </PageHeaderWrapper>
    );
};

JobCreate.propTypes = {
    globalConfig: PropTypes.any,
}

export default connect(({ global }) => ({
    globalConfig: global.config
}))(JobCreate);