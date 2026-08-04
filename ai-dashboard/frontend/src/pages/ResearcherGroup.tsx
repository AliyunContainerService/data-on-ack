import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Tag, message, Popconfirm, Card } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined, TeamOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { fetchUserGroups, createUserGroup, deleteUserGroup, type UserGroup } from '@/api/userGroup'
import { fetchQuotaTrees } from '@/api/quota'
import { get, getErrorMessage } from '@/api/client'

export default function ResearcherGroup() {
  const { t } = useTranslation()
  const [groups, setGroups] = useState<UserGroup[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editGroup, setEditGroup] = useState<UserGroup | null>(null)
  const [quotaNames, setQuotaNames] = useState<string[]>([])
  const [roleOptions, setRoleOptions] = useState<string[]>([])
  const [clusterRoleOptions, setClusterRoleOptions] = useState<string[]>([])
  const [form] = Form.useForm()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [groupData, treeData] = await Promise.all([fetchUserGroups(), fetchQuotaTrees()])
      setGroups(groupData || [])
      const names: string[] = []
      ;(treeData || []).forEach((tree: any) => {
        if (tree.spec?.root) collectLeafNames(tree.spec.root, names)
      })
      setQuotaNames(names)
    } catch { /* */ } finally { setLoading(false) }
  }, [])

  const loadRbacOptions = useCallback(async () => {
    try {
      const data: any = await get('/k8s/rbac/options')
      if (data) {
        setRoleOptions(data.roles || [])
        setClusterRoleOptions(data.clusterRoles || [])
      }
    } catch {
      // Fallback: provide common known roles
      setRoleOptions(['kubeai-researcher-role'])
      setClusterRoleOptions(['kubeai-researcher-clusterrole', 'kubeai-admin-clusterrole'])
    }
  }, [])

  useEffect(() => { load(); loadRbacOptions() }, [load, loadRbacOptions])

  const collectLeafNames = (node: any, names: string[]) => {
    if (!node.children || node.children.length === 0) { names.push(node.name); return }
    node.children.forEach((c: any) => collectLeafNames(c, names))
  }

  const handleCreate = () => { setEditGroup(null); form.resetFields(); setModalOpen(true) }

  const handleEdit = (record: UserGroup) => {
    setEditGroup(record)
    form.setFieldsValue({
      groupName: record.spec.groupName,
      quotaNames: record.spec.quotaNames,
      defaultRoles: record.spec.defaultRoles,
      defaultClusterRoles: record.spec.defaultClusterRoles,
    })
    setModalOpen(true)
  }

  const handleDelete = async (groupName: string) => {
    try { await deleteUserGroup(groupName); message.success(t('group.deleted')); load() }
    catch (err) { message.error(getErrorMessage(err)) }
  }

  const handleModalOk = async () => {
    try {
      const values = await form.validateFields()
      await createUserGroup(values)
      message.success(editGroup ? t('group.updated') : t('group.created'))
      setModalOpen(false); load()
    } catch (err: any) {
      if (err?.errorFields) return // form validation error, antd handles display
      message.error(getErrorMessage(err))
    }
  }

  const columns = [
    { title: t('group.groupName'), dataIndex: ['spec', 'groupName'], key: 'groupName' },
    {
      title: t('group.quotaBinding'), dataIndex: ['spec', 'quotaNames'], key: 'quotaNames',
      render: (names: string[]) => names?.length ? names.map((n: string) => <Tag key={n} color="geekblue">{n}</Tag>) : '-',
    },
    {
      title: t('group.defaultRoles'), dataIndex: ['spec', 'defaultRoles'], key: 'defaultRoles',
      render: (roles: string[]) => roles?.length ? roles.map((r: string) => <Tag key={r}>{r}</Tag>) : '-',
    },
    {
      title: t('group.defaultClusterRoles'), dataIndex: ['spec', 'defaultClusterRoles'], key: 'defaultClusterRoles',
      render: (roles: string[]) => roles?.length ? roles.map((r: string) => <Tag key={r} color="purple">{r}</Tag>) : '-',
    },
    {
      title: t('common.actions'), key: 'actions', width: 160, align: 'center' as const,
      render: (_: unknown, record: UserGroup) => (
        <Space size={4}>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          <Popconfirm title={t('common.confirmDelete')} onConfirm={() => handleDelete(record.spec.groupName)}>
            <Button size="small" danger icon={<DeleteOutlined />} disabled={record.spec.groupName === 'default-user-group'} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Card
      size="small"
      title={<Space><TeamOutlined />{t('group.title')}</Space>}
      extra={
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('group.create')}</Button>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading} />
        </Space>
      }
      style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}
    >
      <Table
        dataSource={groups}
        columns={columns}
        rowKey={(r) => r.metadata.name}
        loading={loading}
        size="small"
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />
      <Modal
        title={editGroup ? t('group.edit') : t('group.create')}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText={t('common.confirm')}
        cancelText={t('common.cancel')}
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="groupName" label={t('group.groupName')} rules={[{ required: true }]}>
            <Input disabled={!!editGroup} placeholder="my-team-group" />
          </Form.Item>
          <Form.Item name="quotaNames" label={t('group.quotaBinding')}>
            <Select mode="multiple" placeholder="Select quota leaf nodes..." options={quotaNames.map(n => ({ label: n.split('.').pop(), value: n }))} />
          </Form.Item>
          <Form.Item name="defaultRoles" label={t('group.defaultRoles')}>
            <Select mode="multiple" placeholder="Select namespace roles..." options={roleOptions.map(r => ({ label: r, value: r }))} />
          </Form.Item>
          <Form.Item name="defaultClusterRoles" label={t('group.defaultClusterRoles')}>
            <Select mode="multiple" placeholder="Select cluster roles..." options={clusterRoleOptions.map(r => ({ label: r, value: r }))} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
