import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Space, Modal, Form, Select, Tag, message, Tooltip, Card, Typography } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, DownloadOutlined, CopyOutlined, ReloadOutlined, UserAddOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  fetchResearchers, createResearcher, updateResearcher, deleteResearcher,
  getBearerToken, downloadKubeConfig, fetchRamUsers, type K8sUser, type RamUser,
} from '@/api/user'
import { get, getErrorMessage } from '@/api/client'

const { Text } = Typography

export default function ResearcherList() {
  const { t } = useTranslation()
  const [users, setUsers] = useState<K8sUser[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editUser, setEditUser] = useState<K8sUser | null>(null)
  const [form] = Form.useForm()

  // RAM users for selection
  const [ramUsers, setRamUsers] = useState<RamUser[]>([])
  const [ramLoading, setRamLoading] = useState(false)

  // User groups for selection
  const [groupOptions, setGroupOptions] = useState<string[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchResearchers()
      setUsers(data?.items || [])
    } catch { /* */ } finally { setLoading(false) }
  }, [])

  const loadGroups = useCallback(async () => {
    try {
      const data: any = await get('/user_group/list')
      const items = Array.isArray(data) ? data : data?.items || []
      setGroupOptions(items.map((g: any) => g?.spec?.groupName || g?.metadata?.name || '').filter(Boolean))
    } catch { /* */ }
  }, [])

  useEffect(() => { load(); loadGroups() }, [load, loadGroups])

  const loadRamUsers = async () => {
    setRamLoading(true)
    try {
      const data = await fetchRamUsers()
      setRamUsers(data || [])
    } catch { setRamUsers([]) } finally { setRamLoading(false) }
  }

  const handleCreate = () => {
    setEditUser(null)
    form.resetFields()
    form.setFieldsValue({ apiRoles: ['researcher'] })
    setModalOpen(true)
    // Load RAM users for selection
    loadRamUsers()
  }

  const handleEdit = (record: K8sUser) => {
    setEditUser(record)
    form.setFieldsValue({
      userName: record.spec.userName,
      aliuid: record.spec.aliuid,
      apiRoles: record.spec.apiRoles,
      groups: record.spec.groups,
    })
    setModalOpen(true)
  }

  const handleDelete = async (userId: string) => {
    try { await deleteResearcher(userId); message.success(t('user.deleted')); load() }
    catch (err) { message.error(getErrorMessage(err)) }
  }

  const handleToken = async (userId: string) => {
    try {
      const token = await getBearerToken(userId)
      // clipboard API requires HTTPS; fallback to textarea copy for HTTP
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(token)
      } else {
        const textarea = document.createElement('textarea')
        textarea.value = token
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }
      message.success(t('user.tokenCopied'))
    } catch (err) { message.error(getErrorMessage(err)) }
  }

  const handleKubeConfig = async (userId: string) => {
    try {
      const blob = await downloadKubeConfig(userId)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${userId}-kubeconfig`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) { message.error(getErrorMessage(err)) }
  }

  // When a RAM user is selected from dropdown, auto-fill aliuid and userName
  const handleRamUserSelect = (value: string) => {
    const selected = ramUsers.find((u) => u.userName === value)
    if (selected) {
      form.setFieldsValue({ aliuid: selected.userId, userName: selected.userName, selectedRamUser: value })
    }
  }

  const handleModalOk = async () => {
    try {
      const values = await form.validateFields()
      const userName = values.userName || values.selectedRamUser
      if (!userName) {
        message.error(t('user.selectRamPlaceholder'))
        return
      }
      const data = {
        userName,
        aliuid: values.aliuid || '',
        apiRoles: values.apiRoles || [],
        groups: values.groups || [],
        k8sServiceAccount: editUser?.spec.k8sServiceAccount || undefined,
      }
      if (editUser) { await updateResearcher(data); message.success(t('user.updated')) }
      else { await createResearcher(data); message.success(t('user.created')) }
      setModalOpen(false); load()
    } catch (err: any) {
      const errMsg = err?.message || err?.msg || (typeof err === 'string' ? err : '')
      if (errMsg) message.error(errMsg)
    }
  }

  // Filter out RAM users already registered
  const existingNames = new Set(users.map(u => u.spec.userName))
  const availableRamUsers = ramUsers.filter(u => !existingNames.has(u.userName))

  const columns = [
    { title: t('user.userName'), dataIndex: ['spec', 'userName'], key: 'userName' },
    {
      title: t('user.role'), dataIndex: ['spec', 'apiRoles'], key: 'apiRoles',
      render: (roles: string[]) => roles?.map((r: string) => <Tag key={r} color={r === 'admin' ? 'red' : 'blue'}>{r}</Tag>),
    },
    {
      title: t('user.groups'), dataIndex: ['spec', 'groups'], key: 'groups',
      render: (groups: string[]) => groups?.length ? groups.map((g: string) => <Tag key={g} color="geekblue">{g}</Tag>) : '-',
    },
    {
      title: t('user.aliuid'), dataIndex: ['spec', 'aliuid'], key: 'aliuid',
      render: (uid: string) => uid ? <Text code style={{ fontSize: 11 }}>{uid}</Text> : '-',
    },
    {
      title: t('common.actions'), key: 'actions', width: 240, align: 'center' as const,
      render: (_: unknown, record: K8sUser) => (
        <Space size={4}>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Tooltip title={t('user.kubeconfig')}>
            <Button size="small" icon={<DownloadOutlined />} onClick={() => handleKubeConfig(record.metadata.name)} />
          </Tooltip>
          <Tooltip title={t('user.token')}>
            <Button size="small" icon={<CopyOutlined />} onClick={() => handleToken(record.metadata.name)} />
          </Tooltip>
          <Tooltip title={t('common.delete')}>
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.metadata.name)}
              disabled={record.spec.apiRoles?.includes('admin')} />
          </Tooltip>
        </Space>
      ),
    },
  ]

  return (
    <Card
      size="small"
      title={<Space><UserAddOutlined />{t('user.title')}</Space>}
      extra={
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('user.create')}</Button>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading} />
        </Space>
      }
      style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}
    >
      <Table
        dataSource={users}
        columns={columns}
        rowKey={(r) => r.metadata.name}
        loading={loading}
        size="small"
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (total) => `${total}` }}
      />
      <Modal
        title={editUser ? t('user.edit') : t('user.create')}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText={t('common.confirm')}
        cancelText={t('common.cancel')}
        width={520}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {!editUser ? (
            <>
              {/* Create mode: select from RAM user list */}
              <Form.Item
                name="selectedRamUser"
                label={<Space>{t('user.userName')}<Text type="secondary" style={{ fontSize: 11, fontWeight: 400 }}>({t('user.selectRam')})</Text></Space>}
                rules={[{ required: true, message: t('user.selectRamPlaceholder') }]}
              >
                <Select
                  showSearch
                  loading={ramLoading}
                  placeholder={t('user.selectRamPlaceholder')}
                  optionFilterProp="label"
                  onChange={handleRamUserSelect}
                  options={availableRamUsers.map((u) => ({
                    label: `${u.userName}${u.displayName ? ' (' + u.displayName + ')' : ''}`,
                    value: u.userName,
                  }))}
                  notFoundContent={ramLoading ? t('common.loading') : t('common.noData')}
                  style={{ width: '100%' }}
                />
              </Form.Item>
              <Form.Item name="userName" hidden>
                <Select />
              </Form.Item>
              <Form.Item name="aliuid" hidden>
                <Select />
              </Form.Item>
            </>
          ) : (
            <>
              {/* Edit mode: show userName readonly */}
              <Form.Item name="userName" label={t('user.userName')} rules={[{ required: true }]}>
                <Select disabled options={[{ label: editUser.spec.userName, value: editUser.spec.userName }]} />
              </Form.Item>
              <Form.Item name="aliuid" label={t('user.aliuid')}>
                <Select disabled options={[{ label: editUser.spec.aliuid || '-', value: editUser.spec.aliuid || '' }]} />
              </Form.Item>
            </>
          )}
          <Form.Item name="apiRoles" label={t('user.role')}>
            <Select mode="multiple" options={[{ label: 'admin', value: 'admin' }, { label: 'researcher', value: 'researcher' }]} />
          </Form.Item>
          <Form.Item name="groups" label={t('user.groups')}>
            <Select
              mode="multiple"
              placeholder={t('user.groups')}
              options={groupOptions.map(g => ({ label: g, value: g }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
