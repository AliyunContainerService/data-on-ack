import {
    DownOutlined,
    ExclamationCircleFilled,
    MinusOutlined,
    PlusOutlined,
    QuestionCircleTwoTone
} from "@ant-design/icons";
import {
    Alert,
    Button,
    Card,
    Col,
    Divider,
    Dropdown,
    Form,
    Input,
    InputNumber,
    Menu,
    message,
    Row,
    Select,
    Switch,
    Tabs,
    Radio,
    DatePicker,
    Space
} from "antd";
import React, { useState,  useEffect, Fragment } from "react";
import {connect} from "dva";
import {PageHeaderWrapper} from "@ant-design/pro-layout";
import {getDatasources, listPVC, newNotebookSource} from "./service";
import Tooltip from "antd/es/tooltip";
import FooterToolbar from "../../components/FooterToolbar";
import {getLocale, history, useIntl} from 'umi';
import {queryCurrentUser} from "@/services/global";
import * as ComForm from "../../components/Form";
import styles from "./style.less";
var notebookImage = "registry.cn-beijing.aliyuncs.com/kube-ai/jupyter-scipy:1.0.0-8bb314b3"
const NotebookCreate = ({ globalConfig }) => {
    const defaultWorkingDir = '/root/';
    const defaultRelativeCodePath = "code/";
    const intl = useIntl();
    const defaultImages = [
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/jupyter:4.0.5-3.2304",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/jupyter-pytorch:4.0.5-3.2304-2.1.2",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/jupyter-pytorch-cuda:4.0.5-3.2304-2.1.2cu118",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/jupyter-pytorch-cuda:4.0.5-3.2304-2.1.2cu121",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304-cu12.1.1-devel",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304-cu11.8.0-devel",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304-cu12.1.1-cudnn8-devel",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304-cu11.8.0-cudnn8-devel",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304-torch2.2.0.1-cu12.1.1-cudnn8-devel",
        "ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304-torch2.2.0.1-cu11.8.0-cudnn8-devel"
        // "registry.cn-beijing.aliyuncs.com/kube-ai/jupyter:v1.4",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/jupyter-scipy:v1.4",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/jupyter-pytorch-full:v1.4",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/jupyter-pytorch-cuda-full:v1.4",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/tensorflow-notebook:1.15-cpu-py36-ubuntu18.04",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/tensorflow-notebook:1.15-gpu-py36-ubuntu18.04",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/tensorflow-notebook:2.5-cpu-py36-ubuntu18.04",
        // "registry.cn-beijing.aliyuncs.com/kube-ai/tensorflow-notebook:2.5-gpu-py36-ubuntu18.04",
        // "registry.cn-beijing.aliyuncs.com/acs/vscode:ubuntu-18.04",
        // "registry.cn-beijing.aliyuncs.com/acs/vscode:tensorflow-1.15.5-gpu",
        // "registry.cn-beijing.aliyuncs.com/acs/vscode:tensorflow-2.12.0-gpu",
        // "registry.cn-beijing.aliyuncs.com/acs/vscode:pytorch-1.10.0-cuda11.3-cudnn8-devel",
        // "registry.cn-beijing.aliyuncs.com/acs/vscode:cuda-11.2.0-devel-ubuntu18.04",
        // "registry.cn-beijing.aliyuncs.com/acs/vscode:cuda-10.0-cudnn7-devel-ubuntu18.04"
    ]
    window.defaultImages = defaultImages;
    const [submitLoading, setSubmitLoading] = useState(false);
    const [activeTabKey, setActiveTabKey] = useState("Worker");
    const [dataSource, setDataSource] = useState([]);
    const [codeSource, setCodeSource] = useState([]);
    const [namespaces, setNamespaces] = useState([]);
    const [pvcs, setPvcs] = useState([]);
    const [pvcLoading, setPvcLoading] = useState(true);
    const [isLoading, setIsLoading] = useState(false);

    const [notebookImages, setNotebookImages] = useState([])
    const [newNotebookImage, setNewNotebookImage] = useState("")
    const [configNamespace, setConfigNamespace] = useState("")

    const region = location.hostname.split(".")[2] || "cn-hangzhou";
    const [form] = Form.useForm();
    const [cloneInfo, setCloneInfo] = useState(undefined);
    const [usersInfo, setUsersInfo] = useState({});

    const getRandomToken = (length) => {
        if (length > 0) {
            let data = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"];
            let token = "";
            for (let i = 0; i < length; i++) {
                let r = parseInt(Math.random() * 25);
                token += data[r];
            }
            return token;
        } else {
            return false;
        }
    }

    if (sessionStorage.getItem("job")) {
        setCloneInfo(JSON.parse(sessionStorage.getItem("job")));
        sessionStorage.removeItem('job');
    }

    const formInitialTF = {
        name: "",
        image: "",
        tokenSetting: {
            enabled: false,
            token: getRandomToken(16),
        },
        workspaceVolume: {
            enabled: false,
            name: "",
            path: "/home/jovyan"
        },
        privateGit: {
            enabled: false,
            user: "",
            password: "",
        },
        tasks: [
            {
                role: "Worker",
                resource: {
                    gpu: 0,
                    cpu: 0.5,
                    memory: 1
                },
            }
        ],
        dataSource: null,
        imgPullSecrets: null,
        namespaces: '',
        notebookType: "Jupyter",
    };

    const setImagesStorage = (imagesArr) => {
        if(!window.localStorage){
            console.warn("localstorage not supported!");
        }else{
            let storage=window.localStorage;
            let imagesStr = imagesArr.toString()
            storage.setItem("images", imagesStr);
        }
    }

    const getImagesStorage = () => {
        if(!window.localStorage){
            console.warn("localstorage not supported!");
            return notebookImages.toString()
        }else{
            let storage=window.localStorage;
            let res = storage.getItem("images")
            if (res != null){
                return res
            }else {
                return notebookImages.toString()
            }
        }
    }

    const fetchNotebookImages = async () => {
        if(!window.localStorage){
            console.warn("localstorage not supported!");
            setNotebookImages(defaultImages)
        }else{
            let storage=window.localStorage;
            let res = storage.getItem("images")
            if (res != null){
                setNotebookImages(getImagesStorage().split(","))
            }else {
                setImagesStorage(defaultImages)
                setNotebookImages(defaultImages)
            }
        }
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

    const fetchPVC = async (namespaceValue) => {
        setPvcLoading(true);
        const pvcs = await listPVC(namespaceValue)
        if (pvcs.code != "200") {
            setPvcs([])
        }else {
            if (pvcs && pvcs.data) {
                setPvcs(pvcs.data)
            }
        }
        setPvcLoading(false);
    };

    useEffect(() => {
        fetchUser();
        fetchNotebookImages();
        //fetchPVC();
    }, []);

    const onFormSubmit = async form => {
        setIsLoading(true);

        let token = ""
        if (form.tokenSetting.enabled){
            token = form.tokenSetting.token
        }

        let volumes = [];
        if(form.workspaceVolume.enabled){
            let temp = {
                "name": form.workspaceVolume.name,
                "path": form.workspaceVolume.path
            }
            volumes.unshift(temp)
        }

        const dataSourceNameObj ={};
        dataSource.forEach(({name,...other})=>{
            dataSourceNameObj[name] = {...other, name};
        })
        if(form.dataSource && form.dataSource.length >0) {
            form.dataSource.forEach(({dataSource}) => {
                if (dataSourceNameObj[dataSource]) {
                    let {name, pvc_name, local_path, namespace} = dataSourceNameObj[dataSource];
                    let pvcPath = local_path;
                    let temp = {
                        "name": pvc_name,
                        "path": pvcPath
                    }
                    volumes.push(temp)
                }
            })
        }
        let verifySet = new Set()
        for (let index = 0;index < volumes.length;index++) {
            verifySet.add(volumes[index].name)
        }
        if (verifySet.size < volumes.length) {
            alert(intl.formatMessage({id: 'Multiple PVCs with the same name are mounted'}))
            setIsLoading(false)
            return
        }
        const data = {
            name: form.name,
            namespace: form.namespaces,
            image: notebookImage,
            cpus: String(form.tasks[0].resource.cpu),
            gpus: String(form.tasks[0].resource.gpu),
            memory: String(form.tasks[0].resource.memory) + "Gi",
            imagePullPolicy : "IfNotPresent",
            volumes : volumes,
            userId: usersInfo.loginId,
            userName : usersInfo.loginName,
            imagePullSecrets : [],
            token: token,
            notebookType: form.notebookType
        }

        if (form.imgPullSecrets !== undefined && form.imgPullSecrets !== null && form.imgPullSecrets.length !== 0) {
            data.imagePullSecrets = [form.imgPullSecrets]
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

        if (form.tolerations) {
            data.tolerates = form.tolerations.filter(d => d.key).reduce((p,c) => {
                p[c.key] = {};
                p[c.key].operator = c.operator;
                p[c.key].value = c.value;
                p[c.key].effect = c.effect;
                return p
            }, {})
        }

        newNotebookSource(data).then(res => {
            if (res.code === '200') {
                message.success(intl.formatMessage({id: 'dlc-dashboard-add-success'}));
            } else {
                message.error(res.data);
            }
            setIsLoading(false);
            history.push({
                pathname: `/notebooks`,
                query: {}
            });
        }).catch(err => {
            setIsLoading(false);
        });

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
                    cpu: 0.5,
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

    const formItemLayout = {
        labelCol: { span: getLocale() === 'zh-CN' ? 4 : 8 },
        wrapperCol: { span: getLocale() === 'zh-CN' ? 20 : 16 }
    };

    function ImageChange(value) {
        notebookImage = value
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

    const onNotebookImageChange = (v) => {
        setNewNotebookImage(v.target.value)
    }

    const addNotebookImage = () => {
        const tmpNotebookImages = [...notebookImages, newNotebookImage].filter((v, i, self)=>self.indexOf(v) === i)
        setNotebookImages(tmpNotebookImages)
        setImagesStorage(tmpNotebookImages)
        setNewNotebookImage('')
    }

    const deleteNotebookImage = () =>{
        const tmpNotebookImages = notebookImages.filter(x=>x!=newNotebookImage)
        setNotebookImages(tmpNotebookImages)
        setImagesStorage(tmpNotebookImages)
        setNewNotebookImage('')
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

    const namespaceChange = (v) => {
        const currentFormInitial = form.getFieldsValue() || {};
        currentFormInitial.namespaces = v;
        currentFormInitial.workspaceVolume.name = "";
        form.setFieldsValue(currentFormInitial);
        setConfigNamespace(v)
        fetchPVC(v).then()
        fetchDataSource(v).then()
    }
    function pvcSelection(pvc) {
        if (pvc.isBound){
            return(
                <Select.Option title={pvc.name} value={pvc.name}>
                    {pvc.name}
                    <Tooltip title={intl.formatMessage({id: 'Has been mounted to another Pod'})} >
                        <ExclamationCircleFilled twoToneColor="#faad14" />
                    </Tooltip>
                </Select.Option>
            );
        }else {
            return (
                <Select.Option title={pvc.name} value={pvc.name}>
                    {pvc.name}
                </Select.Option>
            )
        }
    }

    const notebookTypeChange = (v) => {
        const currentFormInitial = form.getFieldsValue() || {};
        currentFormInitial.notebookType = v.target.value;
        form.setFieldsValue(currentFormInitial);
    }

    const handleChange = async (value) =>  {
        const dataSource = await getDatasources()
        let newDataSource = [];
        if (dataSource && dataSource.data) {
            for (let key in dataSource.data) {
                if (dataSource.data[key].pvc_name !== value && dataSource.data[key].namespace === configNamespace){
                    newDataSource.push(dataSource.data[key]);
                }
            }
        }
        setDataSource(newDataSource);
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
                        <Card style={{ marginBottom: 12 }} title={intl.formatMessage({id: 'Notebook message'})}>
                            <Form.Item
                                name="name"
                                label={intl.formatMessage({id: 'Notebook Name'})}
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'dlc-dashboard-notebook-name-required'})},
                                    {
                                        pattern: /^[a-z][-a-z0-9]{0,28}[a-z0-9]$/,
                                        message: intl.formatMessage({id: 'dlc-dashboard-job-name-required-rules'})
                                    }
                                ]}
                            >
                                <Input />
                            </Form.Item >
                            <Form.Item
                                shouldUpdate
                                name="image"
                                form={form}
                                label={(
                                    <Tooltip title={(
                                        <div>
                                            <span>{intl.formatMessage({id: 'Notebook Custom Image Tooltip'})}</span>
                                            <a href="https://help.aliyun.com/document_detail/285629.html#title-tng-lqy-yb6" target="_blank" rel="noreferrer">{intl.formatMessage({id: 'Notebook Custom Image Document'})}</a>
                                        </div>)} >
                                        {intl.formatMessage({id: 'Notebook Image'})} <QuestionCircleTwoTone twoToneColor="#faad14"/>
                                    </Tooltip>
                                )}
                                rules={[{ required: true }]}
                            >
                                <Select
                                    onChange={ImageChange}
                                    placeholder="custom dropdown render"
                                    style={{ width: '100%px' }}
                                    dropdownRender={menu => (
                                        <div>
                                            {menu}
                                            <Divider style={{ margin: '4px 0' }} />
                                            <Form.Item rules={[{required: true}]}>
                                                <div style={{ display: 'flex', flexWrap: 'nowrap', padding: 8 }}>
                                                    <Input style={{ flex: 'auto' }} value={newNotebookImage}  onChange={onNotebookImageChange}/>
                                                    <a
                                                        style={{ flex: 'none', padding: '8px', display: 'block', cursor: 'pointer' }}
                                                        onClick={addNotebookImage}
                                                    >
                                                        <PlusOutlined /> {intl.formatMessage({id: 'dlc-notebook-add-customized-image'})}
                                                    </a>
                                                    <a
                                                        style={{ flex: 'none', padding: '8px', display: 'block', cursor: 'pointer' }}
                                                        onClick={deleteNotebookImage}
                                                    >
                                                        <MinusOutlined /> {intl.formatMessage({id: 'dlc-notebook-delete-customized-image'})}
                                                    </a>
                                                </div>
                                            </Form.Item>
                                        </div>
                                    )}
                                >
                                    {notebookImages.map(item => (
                                        item &&
                                        <Select.Option title={item} key={item} value={item}>{item}</Select.Option>
                                    ))}
                                </Select>
                            </Form.Item>
                            <Form.Item
                                shouldUpdate
                                noStyle
                            >
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
                            <ComForm.FromAddDropDown
                                // onChange={changeVisible}
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
                                noStyle
                            >
                                {() =>
                                    (<Form.Item label="Workspace PVC">
                                            <Form.Item
                                                name={["workspaceVolume", "enabled"]}
                                                valuePropName="checked"
                                            >
                                                <Switch />
                                            </Form.Item>
                                            {form.getFieldValue(["workspaceVolume", "enabled"]) === true &&
                                            <React.Fragment>
                                                <Form.Item
                                                    label={(
                                                        <Tooltip title={intl.formatMessage({id: 'Workspace Mount PVC, Must be readable and writable'})} >
                                                            {intl.formatMessage({id: 'Target PVC'})} <QuestionCircleTwoTone twoToneColor="#faad14" />
                                                        </Tooltip>
                                                    )}
                                                    name={["workspaceVolume", "name"]}
                                                    rules={[
                                                        { required: true, message: intl.formatMessage({id: 'dlc-notebook-workspace-dir-rules'})}
                                                    ]}
                                                    labelCol={{ span: 10 }}
                                                    wrapperCol={{ span: 18 }}
                                                >
                                                    <Select value={form.getFieldsValue(["workspaceVolume", "name"])} onChange={handleChange}>

                                                        {pvcs.map(pvc => (

                                                            // <Select.Option title={pvc.name} value={pvc.name}>
                                                            //   {pvc.name}
                                                            // </Select.Option>
                                                            pvcSelection(pvc)
                                                        ))}
                                                    </Select>
                                                    {/* <Input placeholder={'/root/data/log/'}/> */}
                                                </Form.Item>

                                                <Form.Item
                                                    label={(
                                                        <Tooltip title={intl.formatMessage({id: 'Workspace Mount PVC Path'})} >
                                                            {intl.formatMessage({id: 'Target path'})} <QuestionCircleTwoTone twoToneColor="#faad14" />
                                                        </Tooltip>
                                                    )}
                                                    name={["workspaceVolume", "path"]}
                                                    rules={[
                                                        { required: true, message: intl.formatMessage({id: 'dlc-dashboard-workspace-dir-rules'})}
                                                    ]}
                                                    labelCol={{ span: 10 }}
                                                    wrapperCol={{ span: 18 }}
                                                >
                                                    <Input placeholder={'/home/jovyan'} defaultValue="/home/jovyan" />
                                                </Form.Item>
                                            </React.Fragment>}
                                        </Form.Item>
                                    )}
                            </Form.Item>

                            <Form.Item
                                shouldUpdate
                                noStyle
                            >
                                {() =>
                                    (<Form.Item label="Token">
                                            <Form.Item
                                                name={["tokenSetting", "enabled"]}
                                                valuePropName="checked"
                                            >
                                                <Switch />
                                            </Form.Item>
                                            {form.getFieldValue(["tokenSetting", "enabled"]) === true &&
                                            <React.Fragment>
                                                <Form.Item
                                                    label={intl.formatMessage({id: 'Token'})}
                                                    name={["tokenSetting", "token"]}
                                                    rules={[
                                                        { required: true, message: intl.formatMessage({id: 'token content required'})},
                                                        {
                                                            pattern: /^[a-z][-a-z0-9]{0,28}[a-z0-9]$/,
                                                            message: intl.formatMessage({id: 'dlc-dashboard-job-name-required-rules'})
                                                        }
                                                    ]}
                                                    labelCol={{ span: 10 }}
                                                    wrapperCol={{ span: 18 }}
                                                >
                                                    <Input defaultValue={getRandomToken(16)} />
                                                </Form.Item>
                                            </React.Fragment>}
                                        </Form.Item>
                                    )}
                            </Form.Item>

                            <Form.Item
                                label="Notebook Type"
                                name={["notebookType"]}
                            >
                                <Radio.Group onChange={notebookTypeChange}>
                                    <Radio value="Jupyter">Jupyter</Radio>
                                    <Radio value="VSCode">VSCode</Radio>
                                </Radio.Group>
                            </Form.Item>

                        </Card>
                    </Col>
                    <Col span={11}>
                        <Card title={intl.formatMessage({id: 'Notebook configure'})} style={{ marginBottom: 12 }}>
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
                                            <Tabs.TabPane
                                                tab={"Notebook"}
                                                key={form.getFieldValue("tasks")[idx].role}
                                                closable={
                                                    form.getFieldValue("tasks")[idx].role !== "Worker"}>
                                                <Form.Item
                                                    name={[field.name, "resource", "cpu"]}
                                                    label={intl.formatMessage({id: 'dlc-dashboard-cpu'})}
                                                    fieldKey={[field.fieldKey, "resource", "cpu"]}>
                                                    <InputNumber
                                                        min={0.5}
                                                        max={96}
                                                        step={0.1}
                                                        // precision={0}
                                                        defaultValue={0.5}
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
                                            </Tabs.TabPane>
                                        ))}
                                    </Tabs>
                                )}
                            </Form.List>
                        </Card>
                        <Card title={intl.formatMessage({id: 'dlc-dashboard-training-advance-config'})} style={{ marginBottom: 12 }}>
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
                    <Button type="primary" htmlType="submit" loading={isLoading}>
                        {intl.formatMessage({id: 'Create notebook'})}
                    </Button>
                </FooterToolbar>
            </Form>
        </PageHeaderWrapper>
    );
};

export default connect(({ global }) => ({
    globalConfig: global.config
}))(NotebookCreate);
