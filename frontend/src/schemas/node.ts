import { z } from 'zod';

export const NodeRecordSchema = z.object({
  id: z.number(),
  name: z.string().optional(),
  remark: z.string().optional(),
  scheme: z.string().optional(),
  address: z.string().optional(),
  port: z.number().optional(),
  basePath: z.string().optional(),
  apiToken: z.string().optional(),
  enable: z.boolean().optional(),
  status: z.string().optional(),
  latencyMs: z.number().optional(),
  cpuPct: z.number().optional(),
  memPct: z.number().optional(),
  xrayVersion: z.string().optional(),
  panelVersion: z.string().optional(),
  uptimeSecs: z.number().optional(),
  inboundCount: z.number().optional(),
  clientCount: z.number().optional(),
  onlineCount: z.number().optional(),
  depletedCount: z.number().optional(),
  lastHeartbeat: z.number().optional(),
  lastError: z.string().optional(),
  allowPrivateAddress: z.boolean().optional(),
  tlsVerifyMode: z.enum(['verify', 'skip', 'pin']).optional(),
  pinnedCertSha256: z.string().optional(),
}).loose();

export const NodeListSchema = z.array(NodeRecordSchema);

export const ProbeResultSchema = z.object({
  status: z.string(),
  latencyMs: z.number().optional(),
  xrayVersion: z.string().optional(),
  error: z.string().optional(),
}).loose();

export const NodeBootstrapStepSchema = z.object({
  name: z.string(),
  ok: z.boolean(),
  output: z.string().optional(),
}).loose();

export const NodeBootstrapResultSchema = z.object({
  node: NodeRecordSchema,
  steps: z.array(NodeBootstrapStepSchema),
}).loose();

export const NodeFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  remark: z.string().optional(),
  scheme: z.enum(['http', 'https']),
  address: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  port: z.number().int().min(1).max(65535),
  basePath: z.string(),
  apiToken: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  enable: z.boolean(),
  allowPrivateAddress: z.boolean(),
  tlsVerifyMode: z.enum(['verify', 'skip', 'pin']),
  pinnedCertSha256: z.string().optional().default(''),
});

export const NodeBootstrapFormSchema = z.object({
  name: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  remark: z.string().optional(),
  address: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  sshUser: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  sshPassword: z.string().min(1, 'pages.nodes.toasts.fillRequired'),
  sshPort: z.number().int().min(1).max(65535).default(22),
  agentPort: z.number().int().min(1).max(65535).default(2053),
  bootstrapBase: z.string().optional().default('/'),
});

export type NodeRecord = z.infer<typeof NodeRecordSchema>;
export type ProbeResult = z.infer<typeof ProbeResultSchema>;
export type NodeFormValues = z.infer<typeof NodeFormSchema>;
export type NodeBootstrapStep = z.infer<typeof NodeBootstrapStepSchema>;
export type NodeBootstrapResult = z.infer<typeof NodeBootstrapResultSchema>;
export type NodeBootstrapFormValues = z.infer<typeof NodeBootstrapFormSchema>;
