import { PlusOutlined } from "@ant-design/icons";
import {
  ModalForm,
  ProFormGroup,
  ProFormText,
  ProFormTextArea,
} from "@ant-design/pro-form";
import { Button, Form, message } from "antd";
import React from "react";
import { ModelRegistryService } from "../services";
import { useIntl } from "umi";

function CreateModelVersion({ modelName }) {
  const intl = useIntl();
  const [form] = Form.useForm<{
    name: string;
    source: string;
    description?: string;
  }>();

  const onFinish = async (values) => {
    ModelRegistryService.createModelVersion(
      values.name,
      values.source,
      undefined,
      undefined,
      undefined,
      values?.description
    )
      .then(() => {
        message.success("创建成功");
      })
      .catch(() => {
        message.error("创建失败");
      });

    // 提交之后关闭表单
    return true;
  };

  return (
    <ModalForm<{
      name: string;
      source: string;
      description?: string;
    }>
      title={intl.formatMessage({ id: "New Model Version" })}
      trigger={
        <Button type="primary">
          <PlusOutlined />
          {intl.formatMessage({ id: "New Model Version" })}
        </Button>
      }
      form={form}
      modalProps={{
        destroyOnClose: true,
      }}
      onFinish={onFinish}
    >
      <ProFormGroup>
        <ProFormText
          name="name"
          label={intl.formatMessage({ id: "Model Name" })}
          initialValue={modelName}
          fieldProps={{
            disabled: true,
          }}
        ></ProFormText>
      </ProFormGroup>
      <ProFormGroup>
        <ProFormTextArea
          name="description"
          label={intl.formatMessage({ id: "Model Description" })}
          placeholder={intl.formatMessage({ id: "Input Model Description" })}
          width="xl"
        ></ProFormTextArea>
      </ProFormGroup>
      <ProFormGroup>
        <ProFormText
          name="source"
          label={intl.formatMessage({ id: "Model Source" })}
          placeholder={intl.formatMessage({ id: "Input Model Source" })}
          width="xl"
          rules={[
            {
              required: true,
            },
          ]}
        ></ProFormText>
      </ProFormGroup>
    </ModalForm>
  );
}

export default CreateModelVersion;
