import { PlusOutlined } from "@ant-design/icons";
import {
  ModalForm,
  ProFormGroup,
  ProFormList,
  ProFormText,
  ProFormTextArea,
} from "@ant-design/pro-form";
import { Button, Form, message } from "antd";
import React from "react";
import { ModelRegistryService } from "../services";
import { useIntl } from "umi";
import ModelVersionTagList from "./ModelVersionTagList";
import { ModelVersionTag } from "../types";

function CreateRegisteredModel() {
  const intl = useIntl();
  const [form] = Form.useForm<{
    name: string;
    tags?: ModelVersionTag[];
    descriptions?: string;
  }>();

  return (
    <ModalForm<{
      name: string;
      tags?: ModelVersionTag[];
      description?: string;
    }>
      title={intl.formatMessage({ id: "New Registered Model" })}
      trigger={
        <Button type="primary">
          <PlusOutlined />
          {intl.formatMessage({ id: "New Registered Model" })}
        </Button>
      }
      form={form}
      modalProps={{
        destroyOnClose: true,
      }}
      onFinish={async (values) => {
        ModelRegistryService.createRegisteredModel(
          values.name,
          values?.tags,
          values?.description
        )
          .then((response) => {
            message.success(
              `成功创建注册模型: ${response.registered_model.name}`
            );
          })
          .catch(() => {
            message.error(`创建注册模型失败`);
          });

        // 提交之后关闭表单
        return true;
      }}
    >
      <ProFormGroup>
        <ProFormText
          name="name"
          label={intl.formatMessage({ id: "Model Name" })}
          placeholder={intl.formatMessage({ id: "Input Model Name" })}
          rules={[
            {
              required: true,
            },
          ]}
          width={"xl"}
        />
      </ProFormGroup>
      <ProFormGroup>
        <ProFormList
          name="tags"
          label={intl.formatMessage({ id: "Model Tags" })}
          creatorButtonProps={{
            creatorButtonText: intl.formatMessage({ id: "New Model Tag" }),
          }}
        >
          <ProFormGroup key="tags">
            <ProFormText
              name="key"
              label={intl.formatMessage({ id: "Model Tag Key" })}
              rules={[
                {
                  required: true,
                },
              ]}
            />
            <ProFormText
              name="value"
              label={intl.formatMessage({ id: "Model Tag Value" })}
            />
          </ProFormGroup>
        </ProFormList>
      </ProFormGroup>
      <ProFormGroup>
        <ProFormTextArea
          name="description"
          label={intl.formatMessage({ id: "Model Description" })}
          placeholder={intl.formatMessage({ id: "Input Model Description" })}
          width="xl"
        />
      </ProFormGroup>
    </ModalForm>
  );
}

export default CreateRegisteredModel;
