import { useState, useEffect } from 'react'
import { Card, Form, InputNumber, Input, Button, Space, Typography, message, Divider, Switch, Tag } from 'antd'
import { SaveOutlined, ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get, post, getErrorMessage } from '@/api/client'

const { Title, Text, Paragraph } = Typography

interface PlatformConfig {
  cullingEnabled: boolean
  cullingIdleTime: number     // minutes
  cullingCheckPeriod: number  // minutes
  defaultImages: string[]
  commitRegistry: string
}

export default function Settings() {
  const { t } = useTranslation()
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const fetchConfig = () => {
    setLoading(true)
    get<PlatformConfig>('/ops/settings')
      .then((data: any) => {
        if (data) form.setFieldsValue(data)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchConfig() }, [])

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      setSaving(true)
      await post('/ops/settings', values)
      message.success(t('common.success'))
    } catch (err) {
      message.error(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={5} style={{ margin: 0 }}>{t('menu.settings')}</Title>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchConfig} loading={loading}>{t('common.refresh')}</Button>
          <Button type="primary" icon={<SaveOutlined />} onClick={handleSave} loading={saving}>{t('common.save')}</Button>
        </Space>
      </div>

      <Form form={form} layout="vertical" style={{ maxWidth: 720 }}>
        {/* Idle Culling */}
        <Card bordered={false} style={{ borderRadius: 10, marginBottom: 16 }}>
          <Title level={5}>Notebook Idle Culling</Title>
          <Paragraph type="secondary" style={{ marginBottom: 16 }}>
            Automatically stop idle Notebooks to reclaim GPU/CPU resources.
            The notebook-controller checks kernel activity periodically and stops notebooks that exceed the idle timeout.
          </Paragraph>
          <Form.Item name="cullingEnabled" label="Enable Idle Culling" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Space size={24}>
            <Form.Item name="cullingIdleTime" label="Idle Timeout (minutes)" tooltip="Max idle time before notebook is stopped">
              <InputNumber min={10} max={10080} style={{ width: 160 }} addonAfter="min" />
            </Form.Item>
            <Form.Item name="cullingCheckPeriod" label="Check Interval (minutes)" tooltip="How often to check notebook activity">
              <InputNumber min={1} max={60} style={{ width: 160 }} addonAfter="min" />
            </Form.Item>
          </Space>
        </Card>

        {/* Default Images */}
        <Card bordered={false} style={{ borderRadius: 10, marginBottom: 16 }}>
          <Title level={5}>Default Notebook Images</Title>
          <Paragraph type="secondary" style={{ marginBottom: 16 }}>
            Pre-configured images available in the "Create Notebook" template gallery on the developer console.
            One image per line.
          </Paragraph>
          <Form.Item name="defaultImages">
            <Input.TextArea
              rows={6}
              placeholder={[
                'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-pytorch:2.1-gpu-cuda12.1',
                'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-tensorflow:2.14-gpu-cuda12.1',
                'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-scipy:latest',
              ].join('\n')}
              style={{ fontFamily: 'monospace', fontSize: 12 }}
            />
          </Form.Item>
        </Card>

        {/* Commit Agent Registry */}
        <Card bordered={false} style={{ borderRadius: 10, marginBottom: 16 }}>
          <Title level={5}>Environment Commit Settings</Title>
          <Paragraph type="secondary" style={{ marginBottom: 16 }}>
            Configure the target container registry where commit-agent pushes saved notebook environments.
          </Paragraph>
          <Form.Item name="commitRegistry" label="Target Registry" tooltip="ACR registry address for committed images">
            <Input placeholder="registry.cn-beijing.aliyuncs.com/your-namespace" style={{ fontFamily: 'monospace' }} />
          </Form.Item>
        </Card>
      </Form>
    </div>
  )
}
