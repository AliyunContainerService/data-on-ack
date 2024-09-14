import React, { useState } from "react";
import { Table, Input, Button, Popconfirm, Form } from "antd";
import { EditOutlined, DeleteOutlined } from "@ant-design/icons";
import { RegisteredModel, RegisteredModelTag } from "../types";
import { useIntl } from "umi";
import { ModelRegistryService } from "../services";

interface EditableCellProps extends React.HTMLAttributes<HTMLElement> {
  editing: boolean;
  dataIndex: string;
  title: any;
  inputType: "text";
  record: RegisteredModelTag;
  index: number;
  children: React.ReactNode;
}

const EditableCell: React.FC<EditableCellProps> = ({
  editing,
  dataIndex,
  title,
  inputType,
  record,
  index,
  children,
  ...restProps
}) => {
  const required = dataIndex === "key";
  return (
    <td {...restProps}>
      {editing ? (
        <Form.Item
          name={dataIndex}
          style={{ margin: 0 }}
          rules={[{ required: required, message: `Please Input ${title}!` }]}
        >
          <Input />
        </Form.Item>
      ) : (
        children
      )}
    </td>
  );
};

const EditableRegisteredModelTagsTable = ({ model }) => {
  const [form] = Form.useForm<RegisteredModelTag>();

  const [registeredModel, setRegisteredModel] =
    useState<RegisteredModel>(model);

  const [registeredModelTags, setRegisteredModelTags] = useState<
    RegisteredModelTag[]
  >(model.tags);

  const [editingKey, setEditingKey] = useState<string | null>(null);

  const intl = useIntl();

  const isEditing = (tag: RegisteredModelTag) => tag.key === editingKey;

  const onEdit = (tag: RegisteredModelTag) => {
    form.setFieldsValue({ ...tag });
    setEditingKey(tag.key);
  };

  const onCancel = (key) => {
    if (key === "") {
      const newRegisteredModelTags = registeredModelTags.filter(
        (tag) => tag.key !== ""
      );
      setRegisteredModelTags(newRegisteredModelTags);
    }
    setEditingKey(null);
    form.setFieldsValue({ key: "", value: "" });
  };

  const onSave = async (key) => {
    try {
      const row = await form.validateFields();
      const newRegisteredModelTags = [...registeredModelTags];
      const index = newRegisteredModelTags.findIndex((tag) => tag.key === key);

      if (index > -1) {
        const item = newRegisteredModelTags[index];
        newRegisteredModelTags.splice(index, 1, { ...item, ...row });
        setRegisteredModelTags(newRegisteredModelTags);
      } else {
        newRegisteredModelTags.push(row);
        setRegisteredModelTags(newRegisteredModelTags);
      }

      ModelRegistryService.setRegisteredModelTag(
        registeredModel.name,
        row.key,
        row.value
      );

      setEditingKey(null);
      form.setFieldsValue({ key: "", value: "" });
    } catch (errInfo) {
      console.log("Validate Failed:", errInfo);
    }
  };

  const handleDelete = (key) => {
    const newRegisteredModelTags = registeredModelTags.filter(
      (tag) => tag.key !== key
    );
    setRegisteredModelTags(newRegisteredModelTags);
    setEditingKey(null);
    ModelRegistryService.deleteRegisteredModelTag(registeredModel.name, key);
  };

  const handleAdd = () => {
    const tag: RegisteredModelTag = { key: "", value: "" };
    if (registeredModelTags) {
      setRegisteredModelTags([...registeredModelTags, tag]);
    } else {
      setRegisteredModelTags([tag]);
    }
    setEditingKey("");
  };

  const columns = [
    {
      title: intl.formatMessage({ id: "Model Tag Key" }),
      dataIndex: "key",
      width: "35%",
      editable: true,
    },
    {
      title: intl.formatMessage({ id: "Model Tag Value" }),
      dataIndex: "value",
      width: "35%",
      editable: true,
    },
    {
      title: intl.formatMessage({ id: "Operation" }),
      dataIndex: "operation",
      render: (_, tag: RegisteredModelTag) => {
        const editing = isEditing(tag);
        return editing ? (
          <span>
            <Button onClick={() => onSave(tag.key)}>
              {intl.formatMessage({ id: "Save" })}
            </Button>
            <Button title="Sure to cancel?" onClick={() => onCancel(tag.key)}>
              {intl.formatMessage({ id: "Cancel" })}
            </Button>
          </span>
        ) : (
          <span>
            <Button
              type="link"
              disabled={editingKey !== null}
              onClick={() => onEdit(tag)}
            >
              <EditOutlined /> {intl.formatMessage({ id: "Edit" })}
            </Button>
            <Popconfirm
              title={intl.formatMessage({ id: "Confirm Deletion" })}
              onConfirm={() => handleDelete(tag.key)}
            >
              <Button type="link" disabled={editingKey !== null}>
                <DeleteOutlined /> {intl.formatMessage({ id: "Delete" })}
              </Button>
            </Popconfirm>
          </span>
        );
      },
    },
  ];

  const mergedColumns = columns.map((col) => {
    if (!col.editable) {
      return col;
    }
    return {
      ...col,
      onCell: (record) => ({
        record,
        dataIndex: col.dataIndex,
        title: col.title,
        editing: isEditing(record),
      }),
    };
  });

  return (
    <Form form={form} component={false}>
      <Table<RegisteredModelTag>
        components={{
          body: {
            cell: EditableCell,
          },
        }}
        bordered
        dataSource={registeredModelTags}
        rowClassName="RegisteredModelTag"
        columns={mergedColumns}
      ></Table>
      <Button
        onClick={handleAdd}
        type="primary"
        style={{ marginBottom: 16 }}
        disabled={editingKey !== null}
      >
        {intl.formatMessage({ id: "New Model Tag" })}
      </Button>
    </Form>
  );
};

export default EditableRegisteredModelTagsTable;
