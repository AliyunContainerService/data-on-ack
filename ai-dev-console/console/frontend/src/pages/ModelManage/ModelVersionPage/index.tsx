import { PageContainer, ProDescriptions } from "@ant-design/pro-components";

import React from "react";
import { Tag } from "antd";
import { ModelRegistryService } from "../services";
import { useIntl, useParams } from "umi";
import { Space } from "antd";
import { ModelVersion } from "../types";
import { useState } from "react";
import { useEffect } from "react";
import EditableModelVersionTagsTable from "../components/EditableModelVersionTagsTable";

function ModelVersionPage() {
  const intl = useIntl();
  const params = useParams<{ name: string; version: string }>();
  const { name, version } = params;

  return (
    <div
      style={{
        background: "#FFFFFF",
      }}
    >
      <PageContainer
        header={{
          title: intl.formatMessage({ id: "Model Version Details" }),
          breadcrumb: {},
        }}
        content={
          <>
            <ProDescriptions<ModelVersion>
              bordered
              column={2}
              params={{
                name,
                version,
              }}
              request={async (params) =>
                ModelRegistryService.getModelVersion(
                  params.name,
                  params.version
                )
                  .then((data) => ({
                    success: true,
                    data: data.model_version,
                  }))
                  .catch(() => ({
                    success: false,
                  }))
              }
              columns={[
                {
                  title: intl.formatMessage({ id: "Model Name" }),
                  dataIndex: "name",
                  copyable: true,
                  ellipsis: true,
                  editable: false,
                },
                {
                  title: intl.formatMessage({ id: "Model Version" }),
                  dataIndex: "version",
                  editable: false,
                },
                {
                  title: intl.formatMessage({ id: "Model Creation Timestamp" }),
                  dataIndex: "creation_timestamp",
                  valueType: "dateTime",
                  editable: false,
                },
                {
                  title: intl.formatMessage({
                    id: "Model Last Updated Timestamp",
                  }),
                  dataIndex: "last_updated_timestamp",
                  valueType: "dateTime",
                  editable: false,
                },
                {
                  title: intl.formatMessage({ id: "Model User ID" }),
                  dataIndex: "user_id",
                  hideInDescriptions: true,
                  editable: false,
                  span: 2,
                },
                {
                  title: intl.formatMessage({ id: "Model Source" }),
                  dataIndex: "source",
                  ellipsis: true,
                  copyable: true,
                  editable: false,
                  span: 2,
                },
                {
                  title: intl.formatMessage({ id: "Model Description" }),
                  key: "description",
                  dataIndex: "description",
                  valueType: "textarea",
                  copyable: true,
                  span: 2,
                },
                {
                  title: intl.formatMessage({ id: "Model Tags" }),
                  key: "tags",
                  dataIndex: "tags",
                  editable: false,
                  span: 2,
                  render: (_, modelVersion) =>
                    EditableModelVersionTagsTable({ model: modelVersion }),
                },
                {
                  title: intl.formatMessage({ id: "Model Aliases" }),
                  dataIndex: "aliases",
                  hideInDescriptions: true,
                  editable: false,
                  span: 2,
                  render: (_, record) => (
                    <Space>
                      {record.aliases?.map((alias, _) => (
                        <Tag>{`@${alias}`}</Tag>
                      ))}
                    </Space>
                  ),
                },
              ]}
              editable={{
                onSave: (_key, record, originRow) => {
                  // Update model description
                  if (record.description !== originRow.description) {
                    ModelRegistryService.updateModelVersion(
                      record.name,
                      record.version.toString(),
                      record?.description
                    );
                  }
                  return Promise.resolve();
                },
              }}
            ></ProDescriptions>
          </>
        }
      ></PageContainer>
    </div>
  );
}

export default ModelVersionPage;
