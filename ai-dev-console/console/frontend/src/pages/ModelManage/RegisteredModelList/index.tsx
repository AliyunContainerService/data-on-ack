import ProTable from "@ant-design/pro-table";
import React from "react";
import { ModelRegistryService } from "../services";
import { Link, useIntl, useRouteMatch } from "umi";
import CreateRegisteredModel from "../components/CreateRegisteredModel";
import { Tag } from "antd";
import { Space } from "antd";
import { RegisteredModel } from "../types";

function RegisteredModelList() {
  const intl = useIntl();

  return (
    <ProTable<RegisteredModel>
      headerTitle={intl.formatMessage({ id: "Registered Models Table" })}
      request={async (params: { name }) => {
        let registeredModels: RegisteredModel[] = [];
        let pageToken;

        do {
          try {
            const data = await ModelRegistryService.searchRegisteredModels(
              params.name ? `name LIKE "%${params.name}%"` : "",
              1000,
              undefined,
              pageToken
            );
            registeredModels = [...registeredModels, ...data.registered_models];
            pageToken = data.next_page_token;
          } catch (error) {
            return {
              success: false,
              data: registeredModels,
            };
          }
        } while (pageToken);

        return {
          success: true,
          data: registeredModels,
        };
      }}
      params={{}}
      columns={[
        {
          title: intl.formatMessage({ id: "Model Name" }),
          dataIndex: "name",
          render: (text, record) => {
            const match = useRouteMatch();
            return <Link to={`${match.path}/${record.name}`}>{text}</Link>;
          },
        },
        {
          title: intl.formatMessage({ id: "Model User ID" }),
          dataIndex: "user_id",
          hideInTable: true,
          hideInSearch: true,
        },
        {
          title: intl.formatMessage({ id: "Model Latest Versions" }),
          dataIndex: "latest_versions",
          hideInSearch: true,
          colSize: 0.5,
          render: (_, registeredModel) => {
            const match = useRouteMatch();
            if (registeredModel?.latest_versions?.length > 0) {
              const latest_version = registeredModel.latest_versions[0].version;
              return (
                <Link
                  to={`${match.url}/${registeredModel.name}/versions/${latest_version}`}
                >{`Version ${latest_version}`}</Link>
              );
            }
          },
        },
        {
          title: intl.formatMessage({ id: "Model Description" }),
          dataIndex: "description",
          ellipsis: true,
          copyable: true,
          hideInSearch: true,
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
          hideInTable: true,
          hideInSearch: true,
          render: (_, record) => {
            const match = useRouteMatch();
            return (
              <Space>
                {record.aliases?.map((alias, _) => (
                  <Link
                    to={`${match.path}/${record.name}/versions/${alias.version}`}
                  >{`@${alias.alias}`}</Link>
                ))}
              </Space>
            );
          },
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
      options={{
        fullScreen: true,
        setting: true,
      }}
      toolBarRender={() => [<CreateRegisteredModel></CreateRegisteredModel>]}
    ></ProTable>
  );
}

export default RegisteredModelList;
