import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Row,
  Select,
  Switch,
  message,
} from 'antd';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { Msg } from '@/utils';
import {
  NodeBootstrapFormSchema,
  NodeFormSchema,
  type NodeBootstrapFormValues,
  type NodeBootstrapResult,
  type NodeFormValues,
  type ProbeResult,
} from '@/schemas/node';
import { antdRule } from '@/utils/zodForm';
import './NodeFormModal.css';

type Mode = 'add' | 'edit';

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  testConnection: (payload: Partial<NodeRecord>) => Promise<Msg<ProbeResult>>;
  fetchFingerprint: (payload: Partial<NodeRecord>) => Promise<Msg<string>>;
  save: (payload: Partial<NodeRecord>) => Promise<Msg<unknown>>;
  bootstrap: (payload: NodeBootstrapFormValues) => Promise<Msg<NodeBootstrapResult>>;
  onOpenChange: (open: boolean) => void;
}

type FormValues = NodeFormValues & Partial<NodeBootstrapFormValues>;

function defaultValues(): FormValues {
  return {
    id: 0,
    name: '',
    remark: '',
    scheme: 'https',
    address: '',
    port: 2053,
    basePath: '/',
    apiToken: '',
    enable: true,
    allowPrivateAddress: false,
    tlsVerifyMode: 'verify',
    pinnedCertSha256: '',
    sshUser: '',
    sshPassword: '',
    sshPort: 22,
    agentPort: 2053,
    bootstrapBase: '/',
  };
}

export default function NodeFormModal({
  open,
  mode,
  node,
  testConnection,
  fetchFingerprint,
  save,
  bootstrap,
  onOpenChange,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<FormValues>();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [fetchingPin, setFetchingPin] = useState(false);
  const [testResult, setTestResult] = useState<ProbeResult | null>(null);
  const [bootstrapResult, setBootstrapResult] = useState<NodeBootstrapResult | null>(null);
  const [bootstrapOk, setBootstrapOk] = useState<boolean | null>(null);
  const scheme = Form.useWatch('scheme', form) ?? 'https';
  const tlsVerifyMode = Form.useWatch('tlsVerifyMode', form) ?? 'verify';

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next: NodeFormValues = mode === 'edit' && node
      ? {
        ...base,
        ...(node as unknown as Partial<NodeFormValues>),
        id: node.id,
        scheme: (node.scheme as 'http' | 'https') || base.scheme,
      }
      : base;
    if (next.scheme === 'http') next.tlsVerifyMode = 'skip';
    form.resetFields();
    form.setFieldsValue(next);
    setTestResult(null);
    setBootstrapResult(null);
    setBootstrapOk(null);
  }, [open, mode, node, form]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  function buildPayload(values: NodeFormValues): Partial<NodeRecord> {
    return {
      id: values.id || 0,
      name: values.name.trim(),
      remark: values.remark?.trim() || '',
      scheme: values.scheme,
      address: values.address.trim(),
      port: values.port,
      basePath: values.basePath.trim() || '/',
      apiToken: values.apiToken.trim(),
      enable: values.enable,
      allowPrivateAddress: values.allowPrivateAddress,
      tlsVerifyMode: values.tlsVerifyMode,
      pinnedCertSha256: values.tlsVerifyMode === 'pin' ? values.pinnedCertSha256.trim() : '',
    };
  }

  function buildBootstrapPayload(values: FormValues): NodeBootstrapFormValues {
    const result = NodeBootstrapFormSchema.parse(values);
    return {
      name: result.name,
      remark: result.remark || '',
      address: result.address,
      sshUser: result.sshUser,
      sshPassword: result.sshPassword,
      sshPort: result.sshPort,
      agentPort: result.agentPort,
      bootstrapBase: result.bootstrapBase || '/',
    };
  }

  async function onTest() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await testConnection(payload);
      if (msg?.success && msg.obj) {
        setTestResult(msg.obj);
      } else {
        setTestResult({ status: 'offline', error: msg?.msg || 'unknown error' });
      }
    } finally {
      setTesting(false);
    }
  }

  async function onFetchPin() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
    setFetchingPin(true);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await fetchFingerprint(payload);
      if (msg?.success && msg.obj) {
        form.setFieldValue('pinnedCertSha256', msg.obj);
        messageApi.success(t('pages.nodes.pinFetched'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.pinFetchFailed'));
      }
    } finally {
      setFetchingPin(false);
    }
  }

  async function onFinish(values: FormValues) {
    setSubmitting(true);
    try {
      if (mode === 'edit') {
        const result = NodeFormSchema.safeParse(values);
        if (!result.success) {
          messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
          return;
        }
        const payload = buildPayload(result.data);
        const test = await testConnection(payload);
        const probe = test?.success ? test.obj : null;
        if (!probe || probe.status !== 'online') {
          setTestResult(probe ?? { status: 'offline', error: test?.msg || t('pages.nodes.connectionFailed') });
          return;
        }
        setTestResult(probe);
        const msg = await save(payload);
        if (msg?.success) {
          onOpenChange(false);
        }
        return;
      }

      const payload = buildBootstrapPayload(values);
      const msg = await bootstrap(payload);
      setBootstrapResult(msg?.obj ?? null);
      setBootstrapOk(!!msg?.success);
      if (msg?.success) {
        messageApi.success(msg.msg || t('pages.nodes.bootstrapSuccess'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.bootstrapFailed'));
      }
    } finally {
      setSubmitting(false);
    }
  }

  function close() {
    if (!submitting) onOpenChange(false);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        confirmLoading={submitting}
        okText={mode === 'add' ? (bootstrapResult ? t('close') : t('pages.nodes.bootstrapNode')) : t('save')}
        cancelText={t('cancel')}
        mask={{ closable: false }}
        width="760px"
        onOk={() => {
          if (mode === 'add' && bootstrapResult) {
            close();
            return;
          }
          form.submit();
        }}
        onCancel={close}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={defaultValues()}
          onFinish={onFinish}
          >
            <Row gutter={16}>
              <Col xs={24} md={12}>
                <Form.Item
                  label={t('pages.nodes.name')}
                  name="name"
                  rules={[antdRule(mode === 'add' ? NodeBootstrapFormSchema.shape.name : NodeFormSchema.shape.name, t)]}
                >
                  <Input placeholder={t('pages.nodes.namePlaceholder')} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
              <Form.Item label={t('pages.nodes.remark')} name="remark">
                <Input />
              </Form.Item>
              </Col>
            </Row>

          {mode === 'add' ? (
            <>
              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.address')}
                    name="address"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.address, t)]}
                  >
                    <Input placeholder={t('pages.nodes.addressPlaceholder')} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.sshUser')}
                    name="sshUser"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.sshUser, t)]}
                  >
                    <Input />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.sshPassword')}
                    name="sshPassword"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.sshPassword, t)]}
                  >
                    <Input.Password />
                  </Form.Item>
                </Col>
                <Col xs={12} md={6}>
                  <Form.Item
                    label={t('pages.nodes.sshPort')}
                    name="sshPort"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.sshPort, t)]}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={6}>
                  <Form.Item
                    label={t('pages.nodes.agentPort')}
                    name="agentPort"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.agentPort, t)]}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item label={t('pages.nodes.remark')} name="remark">
                <Input />
              </Form.Item>

              {bootstrapResult && (
                <Alert
                  type={bootstrapOk ? 'success' : 'warning'}
                  showIcon
                  style={{ marginBottom: 16 }}
                  title={bootstrapOk ? t('pages.nodes.bootstrapSuccess') : t('pages.nodes.bootstrapFailed')}
                  description={(
                    <List
                      size="small"
                      dataSource={bootstrapResult.steps}
                      renderItem={(step) => (
                        <List.Item>
                          <div style={{ width: '100%' }}>
                            <div><strong>{step.name}</strong> {step.ok ? t('success') : t('fail')}</div>
                            {step.output ? <pre style={{ margin: '8px 0 0', whiteSpace: 'pre-wrap' }}>{step.output}</pre> : null}
                          </div>
                        </List.Item>
                      )}
                    />
                  )}
                />
              )}
            </>
          ) : (
            <>
              <Row gutter={16}>
                <Col xs={24} md={6}>
                  <Form.Item label={t('pages.nodes.scheme')} name="scheme">
                    <Select
                      options={[
                        { value: 'https', label: 'https' },
                        { value: 'http', label: 'http' },
                      ]}
                      onChange={(value) => {
                        if (value === 'http') form.setFieldValue('tlsVerifyMode', 'skip');
                      }}
                    />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.address')}
                    name="address"
                    rules={[antdRule(NodeFormSchema.shape.address, t)]}
                  >
                    <Input placeholder={t('pages.nodes.addressPlaceholder')} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={6}>
                  <Form.Item
                    label={t('pages.nodes.port')}
                    name="port"
                    rules={[antdRule(NodeFormSchema.shape.port, t)]}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item label={t('pages.nodes.basePath')} name="basePath">
                    <Input placeholder="/" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.enable')}
                    name="enable"
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item
                label={t('pages.nodes.allowPrivateAddress')}
                name="allowPrivateAddress"
                valuePropName="checked"
                extra={t('pages.nodes.allowPrivateAddressHint')}
              >
                <Switch />
              </Form.Item>

              <Form.Item
                label={t('pages.nodes.tlsVerifyMode')}
                name="tlsVerifyMode"
                extra={t('pages.nodes.tlsVerifyModeHint')}
              >
                <Select
                  disabled={scheme === 'http'}
                  options={[
                    { value: 'verify', label: t('pages.nodes.tlsVerify') },
                    { value: 'pin', label: t('pages.nodes.tlsPin') },
                    { value: 'skip', label: t('pages.nodes.tlsSkip') },
                  ]}
                />
              </Form.Item>

              {tlsVerifyMode === 'skip' && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 16 }}
                  title={t('pages.nodes.tlsSkipWarning')}
                />
              )}

              {tlsVerifyMode === 'pin' && (
                <Form.Item
                  label={t('pages.nodes.pinnedCert')}
                  name="pinnedCertSha256"
                  extra={t('pages.nodes.pinnedCertHint')}
                >
                  <Input.Search
                    placeholder={t('pages.nodes.pinnedCertPlaceholder')}
                    enterButton={t('pages.nodes.fetchPin')}
                    loading={fetchingPin}
                    onSearch={onFetchPin}
                  />
                </Form.Item>
              )}

              <Form.Item
                label={t('pages.nodes.apiToken')}
                name="apiToken"
                rules={[antdRule(NodeFormSchema.shape.apiToken, t)]}
                extra={t('pages.nodes.apiTokenHint')}
              >
                <Input.Password placeholder={t('pages.nodes.apiTokenPlaceholder')} />
              </Form.Item>

              <div className="test-row">
                <Button type="default" loading={testing} onClick={onTest}>
                  {t('pages.nodes.testConnection')}
                </Button>
                {testResult && (
                  <div className="test-result">
                    {testResult.status === 'online' ? (
                      <Alert
                        type="success"
                        showIcon
                        title={t('pages.nodes.connectionOk', { ms: testResult.latencyMs })}
                        description={testResult.xrayVersion ? `Xray ${testResult.xrayVersion}` : undefined}
                      />
                    ) : (
                      <Alert
                        type="error"
                        showIcon
                        title={t('pages.nodes.connectionFailed')}
                        description={testResult.error}
                      />
                    )}
                  </div>
                )}
              </div>
            </>
          )}
        </Form>
      </Modal>
    </>
  );
}
