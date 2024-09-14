import { ProFormText } from "@ant-design/pro-components";
import { ProFormGroup } from "@ant-design/pro-components";
import { ProFormList } from "@ant-design/pro-components";
import React from "react";
import { useIntl } from "umi";

function ModelVersionTagList() {
  const intl = useIntl();

  return (
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
  );
}

export default ModelVersionTagList;
