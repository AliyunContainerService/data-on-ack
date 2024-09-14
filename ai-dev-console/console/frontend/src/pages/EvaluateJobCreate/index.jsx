import {
    DownOutlined,
    QuestionCircleTwoTone, ReloadOutlined,
    MinusCircleOutlined, PlusOutlined, MinusOutlined, ExclamationCircleFilled
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
    Divider,
    Switch,
    message,
} from "antd";
import React, { useState,  useEffect } from "react";
import { connect } from "dva";
import { PageHeaderWrapper } from "@ant-design/pro-layout";
import {getDatasources, listPVC, newEvaluateJobSource, getCodeSource} from "./service";
import FooterToolbar from "../../components/FooterToolbar";
import { history, useIntl, getLocale } from 'umi';
import {queryCurrentUser} from "@/services/global";
import * as ComForm from "../../components/Form";
import styles from "./style.less";

var path = require("path");
var globalConfignamespace = ""
var notebookImage = ""
const NotebookCreate = ({ globalConfig }) => {
    const defaultWorkingDir = '/root/';
    const defaultRelativeCodePath = "code/";
    const intl = useIntl();
    const defaultImages = []
    window.defaultImages = defaultImages;
    const [submitLoading, setSubmitLoading] = useState(false);
    const [activeTabKey, setActiveTabKey] = useState("Worker");
    const [dataSource, setDataSource] = useState([]);
    const [codeSource, setCodeSource] = useState([]);
    const [namespaces, setNamespaces] = useState([]);
    const [isLoading, setIsLoading] = useState(false);

    const [notebookImages, setNotebookImages] = useState([])
    const [newNotebookImage, setNewNotebookImage] = useState("")
    const [form] = Form.useForm();
    const [cloneInfo, setCloneInfo] = useState(undefined);
    const [usersInfo, setUsersInfo] = useState({});

    const [defaultModelName, setDefaultModelName] = useState("")
    const [defaultModelVersion, setDefaultModelVersion] = useState("")

    if (sessionStorage.getItem("job")) {
        setCloneInfo(JSON.parse(sessionStorage.getItem("job")));
        sessionStorage.removeItem('job');
    }

    const formInitialTF = {
        modelName: "",
        modelVersion: "",
        name: "",
        image: "",
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
        codeSource: null,
        codeSourceBranch: "",
        // workingDir: defaultWorkingDir,
        command: "",
        modelPath: "",
        datasetPath: "",
        metricsPath: "",
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

    const fetchPVC = async () => {
        setPvcLoading(true);
        let ns = globalConfignamespace
        const pvcs = await listPVC(ns)
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
        let localStorage=window.localStorage;
        let model_version = localStorage.getItem("model_version")
        let model_name = localStorage.getItem("model_name")
        setDefaultModelVersion(model_version)
        setDefaultModelName(model_name)
        fetchSource();
        fetchUser();
        fetchNotebookImages();
        //fetchPVC();
    }, []);

    const onFormSubmit = async form => {
        setIsLoading(true);
        let volumes = {};
        let dataSourceNameObj ={};
        dataSource.forEach(({name,...other})=>{
            dataSourceNameObj[name] = {...other, name};
        })
        if(form.dataSource && form.dataSource.length >0) {
            form.dataSource.forEach(({dataSource}) => {
                if (dataSourceNameObj[dataSource]) {
                    let {name, pvc_name, local_path, namespace} = dataSourceNameObj[dataSource];
                    let pvcPath = local_path;
                    volumes[pvc_name] = pvcPath
                }
            })
        }

        let localStorage=window.localStorage;
        let model_version = localStorage.getItem("model_version")
        let model_name = localStorage.getItem("model_name")

        let data = {
            modelName: model_name,
            modelVersion: model_version,
            name: form.name,
            namespace: form.namespaces,
            image: form.image,
            cpu: String(form.tasks[0].resource.cpu),
            gpu: form.tasks[0].resource.gpu,
            memory: String(form.tasks[0].resource.memory) + "Gi",
            imagePullSecrets : [],
            modelPath: form.modelPath,
            datasetPath: form.datasetPath,
            metricsPath: form.metricsPath,
            command: [form.command],
            // workingDir: form.workingDir,
            dataSources: volumes
        }

        if (form.imgPullSecrets !== undefined && form.imgPullSecrets !== null && form.imgPullSecrets.length !== 0) {
            data.imagePullSecrets = form.imgPullSecrets
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

        newEvaluateJobSource(data).then(res => {
            if (res.code === '200') {
                message.success(intl.formatMessage({id: 'dlc-dashboard-add-success'}));
            } else {
                message.error(res.data);
            }
            setIsLoading(false);
            history.push({
                pathname: `/evaluateJobs`,
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

    const fetchDataSource = async () => {
        let dataSource = await getDatasources();
        let newDataSource = [];
        if (dataSource && dataSource.data) {
            for (let key in dataSource.data) {
                if (dataSource.data[key].namespace == globalConfignamespace) {
                    newDataSource.push(dataSource.data[key]);
                }
            }
        }
        setDataSource(newDataSource)
    }

    const namespaceChange = (v) => {
        const currentFormInitial = form.getFieldsValue() || {};
        currentFormInitial.namespaces = v;
        form.setFieldsValue(currentFormInitial);
        globalConfignamespace = v
        fetchDataSource()
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
                        <Card style={{ marginBottom: 12 }} title= {intl.formatMessage({id: "dlc-evaluateJob-message"})}>
                            <Form.Item
                                name="name"
                                label={intl.formatMessage({id: "dlc-evaluateJob-name"})}
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'evaluateJob name required'})},
                                    {
                                        pattern: /^[a-z][-a-z0-9]{0,28}[a-z0-9]$/,
                                        message: intl.formatMessage({id: 'dlc-dashboard-job-name-required-rules'})
                                    }
                                ]}
                            >
                                <Input />
                            </Form.Item >
                            <Form.Item
                                name="image"
                                label={intl.formatMessage({id: "dlc-evaluateJob-image"})}
                                rules={[{ required: true, message: intl.formatMessage({id: 'evaluateJob image required'}) }]}
                            >
                                <Input />
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
                                                rules={[{ required: true}]}
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
                                shouldUpdate
                                noStyle>
                                {() =>(
                                    <div>
                                        <div className={getLocale() === 'zh-CN' ? styles.gitSourceContainer : styles.gitSourceContainerEn}>
                                            <Form.Item
                                                label= {intl.formatMessage({id: 'dlc-image-pull-secrets'})}
                                                name="imgPullSecrets"
                                                rules={[
                                                    { required: false},
                                                    {
                                                        pattern: /^[a-z][-a-z0-9]{0,28}[a-z0-9]$/,
                                                        message: intl.formatMessage({id: 'dlc-dashboard-job-name-required-rules'})
                                                    }
                                                ]}
                                            >
                                                <Input />
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
                                name="modelPath"
                                label={intl.formatMessage({id: "dlc-model-path"})}
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'Model path required'})},
                                ]}
                            >
                                <Input placeholder={intl.formatMessage({id: 'Model loading path'})}/>
                            </Form.Item >

                            <Form.Item
                                name="datasetPath"
                                label={intl.formatMessage({id: "dlc-dataset-path"})}
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'Dataset path required'})},
                                ]}
                            >
                                <Input placeholder={intl.formatMessage({id: 'Dataset loading path'})} />
                            </Form.Item >

                            <Form.Item
                                name="metricsPath"
                                label={intl.formatMessage({id: "dlc-metrics-path"})}
                                rules={[
                                    { required: true, message: intl.formatMessage({id: 'Metrics path required'})},
                                ]}
                            >
                                <Input placeholder={intl.formatMessage({id: 'Metrics output path'})} />
                            </Form.Item >

                            {/*<Form.Item*/}
                            {/*    name="workingDir"*/}
                            {/*    label={intl.formatMessage({id: "dlc-working-dir"})}*/}
                            {/*    rules={[*/}
                            {/*        { required: true, message: intl.formatMessage({id: 'Metrics path required'})},*/}
                            {/*    ]}*/}
                            {/*>*/}
                            {/*    <Input />*/}
                            {/*</Form.Item >*/}

                            <Form.Item
                                name="command"
                                label={intl.formatMessage({id: 'dlc-dashboard-execute-command'})}
                                rules={[{ required: true, message: intl.formatMessage({id: 'Working dir required'})}]}
                            >
                                <Input.TextArea  placeholder={`python examples/main.py`}/>
                            </Form.Item>

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
                        </Card>
                    </Col>
                    <Col span={11}>
                        <Card title={intl.formatMessage({id: 'dlc-evaluate-config'})} style={{ marginBottom: 12 }}>
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
                                                tab={"Evaluate Job"}
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
                    </Col>
                </Row>
                <FooterToolbar>
                    <Button type="primary" htmlType="submit" loading={isLoading}>
                        提交评测任务
                    </Button>
                </FooterToolbar>
            </Form>
        </PageHeaderWrapper>
    );
};

export default connect(({ global }) => ({
    globalConfig: global.config
}))(NotebookCreate);