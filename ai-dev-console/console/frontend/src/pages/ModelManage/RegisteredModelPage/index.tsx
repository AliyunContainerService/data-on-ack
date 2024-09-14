import { PageHeaderWrapper } from "@ant-design/pro-layout";
import ProTable from "@ant-design/pro-table";
import React from "react";
import { Link, useIntl, useParams, useRouteMatch } from "umi";
import { ModelRegistryService } from "../services";
import CreateModelVersion from "../components/CreateModelVersion";
import { ProDescriptions } from "@ant-design/pro-components";
import { Tag } from "antd";
import { Space } from "antd";
import { ModelVersion, RegisteredModel } from "../types";
import EditableRegisteredModelTagsTable from "../components/EditableRegisteredModelTagsTable";

function RegisteredModelPage() {
  const intl = useIntl();
  const params = useParams<{ name }>();
  const modelName = params.name;

  return (
    <PageHeaderWrapper
      title={intl.formatMessage({ id: "Registered Model Details" })}
    >
      <Space direction="vertical">
        <ProDescriptions<RegisteredModel>
          bordered
          column={1}
          style={{
            background: "#FFFFFF",
          }}
          params={{
            name: modelName,
          }}
          request={async (params) =>
            ModelRegistryService.getRegisteredModel(params.name)
              .then((data) => ({
                success: true,
                data: data.registered_model,
              }))
              .catch(() => ({
                success: false,
              }))
          }
          columns={[
            {
              title: intl.formatMessage({ id: "Model Name" }),
              key: "name",
              dataIndex: "name",
              copyable: true,
              ellipsis: true,
            },
            {
              title: intl.formatMessage({ id: "Model Description" }),
              key: "description",
              dataIndex: "description",
              valueType: "textarea",
              copyable: true,
              ellipsis: true,
              fieldProps: {
                width: "100%",
              },
            },
            {
              title: intl.formatMessage({ id: "Model Tags" }),
              key: "tags",
              dataIndex: "tags",
              editable: false,
              render: (_, registeredModel) =>
                EditableRegisteredModelTagsTable({ model: registeredModel }),
            },
            {
              title: intl.formatMessage({ id: "Model Aliases" }),
              dataIndex: "aliases",
              editable: false,
              render: (_, registeredModel) => {
                const match = useRouteMatch();
                return (
                  <Space direction="vertical">
                    {registeredModel?.aliases?.map((alias, _) => (
                      <Link
                        to={`${match.url}/versions/${alias.version}`}
                      >{`@${alias.alias}`}</Link>
                    ))}
                  </Space>
                );
              },
            },
          ]}
          editable={{
            onSave: async (key, newModel, oldModel) => {
              if (key === "name") {
                if (newModel.name !== oldModel.name) {
                  ModelRegistryService.renameRegisteredModel(
                    oldModel.name,
                    newModel.name
                  )
                    .then((value) => {
                      // history.push();
                    })
                    .catch(() => {});
                }
              } else if (key == "description") {
                if (newModel.description !== oldModel.description) {
                  ModelRegistryService.updateRegisteredModel(
                    params.name,
                    newModel.description
                  );
                }
              }
            },
          }}
        ></ProDescriptions>

        <ProTable
          headerTitle={intl.formatMessage({ id: "Model Versions Table" })}
          request={async () => {
            let modelVersions: ModelVersion[] = [];
            let pageToken;
            do {
              try {
                const data = await ModelRegistryService.searchModelVersions(
                  `name="${modelName}"`,
                  1000,
                  undefined,
                  pageToken
                );
                modelVersions = [...modelVersions, ...data.model_versions];
                pageToken = data.next_page_token;
              } catch (error) {
                return {
                  success: false,
                  data: modelVersions,
                };
              }
            } while (pageToken);

            return {
              success: true,
              data: modelVersions,
            };
          }}
          params={{}}
          columns={[
            {
              title: intl.formatMessage({ id: "Model Name" }),
              dataIndex: "name",
              hideInSearch: true,
              hideInTable: true,
            },
            {
              title: intl.formatMessage({ id: "Model Version" }),
              dataIndex: "version",
              width: "100px",
              colSize: 1,
              render: (text, record) => {
                const match = useRouteMatch();
                return (
                  <Link to={`${match.url}/versions/${record.version}`}>
                    {text}
                  </Link>
                );
              },
            },
            {
              title: intl.formatMessage({ id: "Model User ID" }),
              dataIndex: "user_id",
              hideInTable: true,
              hideInForm: true,
            },
            {
              title: intl.formatMessage({ id: "Model Description" }),
              dataIndex: "description",
              hideInSearch: true,
              hideInTable: true,
              hideInForm: true,
              hideInDescriptions: true,
            },
            {
              title: intl.formatMessage({ id: "Model Source" }),
              dataIndex: "source",
              ellipsis: true,
              hideInSearch: true,
              copyable: true,
              colSize: 4,
            },
            {
              title: intl.formatMessage({ id: "Model Tags" }),
              dataIndex: "tags",
              hideInSearch: true,
              render: (_, record) => (
                <Space direction="vertical">
                  {record.tags?.map((tag, _) => {
                    if (tag.value) {
                      return <Tag>{`${tag.key}: ${tag.value}`}</Tag>;
                    } else {
                      return <Tag>{`${tag.key}`}</Tag>;
                    }
                  })}
                </Space>
              ),
            },
            {
              title: intl.formatMessage({ id: "Model Aliases" }),
              dataIndex: "aliases",
              hideInSearch: true,
              render: (_, record) => (
                <Space>
                  {record.aliases?.map((alias, _) => <Tag>{`${alias}`}</Tag>)}
                </Space>
              ),
            },
            {
              title: intl.formatMessage({ id: "Model Creation Timestamp" }),
              dataIndex: "creation_timestamp",
              valueType: "dateTime",
              hideInSearch: true,
              hideInTable: true,
            },
            {
              title: intl.formatMessage({ id: "Model Last Updated Timestamp" }),
              dataIndex: "last_updated_timestamp",
              valueType: "dateTime",
              hideInSearch: true,
              hideInTable: true,
            },
          ]}
          search={false}
          options={{
            fullScreen: true,
            setting: true,
          }}
          toolBarRender={() => [
            <CreateModelVersion modelName={modelName}></CreateModelVersion>,
          ]}
        ></ProTable>
      </Space>
    </PageHeaderWrapper>
  );
}

export default RegisteredModelPage;
