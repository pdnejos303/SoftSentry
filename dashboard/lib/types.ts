// API response types — mirror backend app/schemas/{machines,software}.py.

export type SignatureStatus = "valid" | "expired" | "invalid" | "unsigned" | null;

export interface VulnerabilityCount {
  critical: number;
  high: number;
  medium: number;
  low: number;
}

// Live scan progress — mirrors backend ScanProgress. null on a machine that is
// idle or has never scanned (no progress bar shown).
export type ScanPhase = "counting" | "scanning" | "verifying" | "uploading" | "idle";

export interface ScanProgress {
  phase: ScanPhase | string;
  done: number;
  total: number;
  current_path: string | null;
  updated_at: string | null;
}

export interface MachineListItem {
  uuid: string;
  hostname: string;
  // Admin-set friendly name + owner; null when unset (UI falls back to hostname).
  display_name: string | null;
  owner: string | null;
  os: string;
  os_version: string;
  agent_version: string;
  status: "online" | "stale" | "offline" | string;
  // Backend URL the agent reports phoning home to; null until first heartbeat
  // from an agent new enough to send it.
  reported_server_url: string | null;
  last_seen_at: string | null;
  last_scan_at: string | null;
  tags: string[];
  software_count: number;
  vulnerability_count: VulnerabilityCount;
  risk_score: number;
  // Live scan progress; null when the machine is idle / has never scanned.
  scan_progress: ScanProgress | null;
}

export interface MachineDetail extends MachineListItem {
  arch: string;
  enrolled_at: string;
}

// Admin edit (PATCH /machines/{uuid}) — partial; omitted fields are left as-is.
export interface MachineUpdateInput {
  display_name?: string | null;
  owner?: string | null;
  tags?: string[];
}

export interface Paginated<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface MachineSoftwareItem {
  uuid: string;
  name: string;
  version: string;
  publisher: string | null;
  install_date: string | null;
  install_path: string | null;
  install_size_kb: number | null;
  arch: string | null;
  source: string;
  signature_status: SignatureStatus;
  is_active: boolean;
  license_status: "compliant" | "over_used" | "expired" | "expiring_soon" | null;
}

export interface SoftwareHistoryItem {
  software_name: string;
  software_version: string;
  event: "installed" | "uninstalled" | "updated" | string;
  previous_version: string | null;
  occurred_at: string;
}

export interface ScanHistoryItem {
  uuid: string;
  started_at: string;
  completed_at: string;
  received_at: string;
  software_count: number;
  scan_type: string;
  trigger: string | null;
}

export interface MachineRef {
  uuid: string;
  name: string;
}

export interface CrossSoftwareItem {
  name: string;
  version: string;
  publisher: string | null;
  installed_count: number;
  machines: MachineRef[];
  signature_status: SignatureStatus;
}

export interface SoftwareStats {
  unique_apps: number;
  total_installs: number;
  valid: number;
  unsigned: number;
  invalid: number;
}

export interface TopSoftwareItem {
  name: string;
  publisher: string | null;
  installed_count: number;
}

export type TrustLevel = "trusted" | "suspicious" | "risky";

export interface RecentInstallItem {
  machine_uuid: string;
  machine_name: string;
  name: string;
  version: string;
  event: "installed" | "updated" | string;
  publisher: string | null;
  detected_at: string; // when the agent first saw it
  installed_at: string | null; // accurate install moment (best-effort)
  install_date: string | null; // registry date-only fallback
  signature_status: SignatureStatus;
  trust: TrustLevel;
}

export interface ListResult<T> {
  items: T[];
  total: number;
}

// ── Module 3: signatures ──────────────────────────────────────────────────────

export interface SignatureListItem {
  software_uuid: string;
  name: string;
  version: string;
  publisher: string | null;
  machine_uuid: string;
  machine_hostname: string;
  status: string;
  // Why verification failed (status="invalid"): reason code or raw HRESULT hex.
  status_reason: string | null;
  signer: string | null;
  cert_valid_to: string | null;
}

export interface CertChainNode {
  subject?: string;
  issuer?: string;
  valid_from?: string;
  valid_to?: string;
  [key: string]: unknown;
}

export interface SignatureDetail {
  software_uuid: string;
  software_name: string;
  software_version: string;
  machine_uuid: string;
  machine_hostname: string;
  status: string;
  // Why verification failed (status="invalid"): reason code or raw HRESULT hex.
  status_reason: string | null;
  signer: string | null;
  issuer: string | null;
  cert_thumbprint: string | null;
  cert_valid_from: string | null;
  cert_valid_to: string | null;
  signature_algorithm: string | null;
  chain: CertChainNode[] | null;
  verified_at: string;
  issues: string[];
}

export interface SignatureStatsItem {
  status: string;
  count: number;
}

export interface SignatureStats {
  total: number;
  distribution: SignatureStatsItem[];
}

// ── Module 4: policy (whitelist / blacklist) + alerts ─────────────────────────

export type Severity = "high" | "medium" | "low";

export interface WhitelistEntry {
  uuid: string;
  name_pattern: string;
  publisher_pattern: string | null;
  version_constraint: string | null;
  notes: string | null;
  created_at: string;
  updated_at: string;
}

export interface BlacklistEntry extends WhitelistEntry {
  severity: Severity;
  reason: string;
}

export interface WhitelistEntryInput {
  name_pattern: string;
  publisher_pattern?: string | null;
  version_constraint?: string | null;
  notes?: string | null;
}

export interface BlacklistEntryInput extends WhitelistEntryInput {
  severity: Severity;
  reason: string;
}

export interface BulkImportError {
  row: number;
  message: string;
}

export interface BulkImportResult {
  imported: number;
  skipped: number;
  errors: BulkImportError[];
}

export interface PolicySettings {
  whitelist_mode: boolean;
  flag_unsigned: boolean;
}

// ── Module 5: vulnerabilities (CVE) ───────────────────────────────────────────

export type VulnSeverity = "critical" | "high" | "medium" | "low";
export type MatchConfidence = "high" | "medium" | "low";

export interface VulnerabilityItem {
  uuid: string;
  cve_id: string;
  severity: string;
  cvss_score: number | null;
  description: string;
  machine_uuid: string;
  machine_hostname: string;
  software_uuid: string;
  software_name: string;
  software_version: string;
  recommended_version: string | null;
  match_confidence: string;
  is_dismissed: boolean;
  dismissed_reason: string | null;
  matched_at: string;
}

export interface VulnerabilityDetail extends VulnerabilityItem {
  references: string[];
  published_at: string | null;
}

export interface SeverityCount {
  severity: string;
  count: number;
}

// One installed software (machine, name, version) with its CVEs aggregated.
export interface VulnerabilityGroup {
  software_uuid: string;
  software_name: string;
  software_version: string;
  machine_uuid: string;
  machine_hostname: string;
  cve_count: number;
  max_cvss: number | null;
  top_severity: string;
  severity_counts: SeverityCount[];
  recommended_version: string | null;
  latest_matched_at: string;
}

export interface VulnSummaryItem {
  severity: string;
  count: number;
}

export interface VulnSummary {
  total: number;
  distribution: VulnSummaryItem[];
}

export interface CveDetail {
  cve_id: string;
  source: string;
  severity: string;
  cvss_score: number | null;
  description: string;
  published_at: string;
  modified_at: string;
  references: string[];
  affected: Array<Record<string, unknown>> | null;
}

export type AlertStatus = "active" | "acknowledged" | "resolved" | string;

export interface Alert {
  uuid: string;
  // Null for org-level alerts (e.g. license expiry) not tied to an endpoint.
  machine_uuid: string | null;
  machine_hostname: string | null;
  software_name: string | null;
  software_version: string | null;
  license_uuid: string | null;
  license_name: string | null;
  type: string;
  severity: Severity | string;
  status: AlertStatus;
  title: string;
  detail: string | null;
  created_at: string;
  acknowledged_at: string | null;
  resolved_at: string | null;
}

// ── Module 6: license compliance ──────────────────────────────────────────────

export type LicenseStatus = "compliant" | "over_used" | "expired" | "expiring_soon";

export interface LicenseItem {
  uuid: string;
  software_name: string;
  publisher: string | null;
  purchased_count: number;
  purchased_at: string | null;
  expires_at: string | null;
  cost_total: number | null;
  currency: string;
  vendor_contact: string | null;
  notes: string | null;
  has_license_key: boolean;
  installed_count: number;
  status: string;
  days_until_expiry: number | null;
  created_at: string;
  updated_at: string;
}

export interface LicenseDetail extends LicenseItem {
  license_key: string | null;
}

export interface LicenseInput {
  software_name: string;
  publisher?: string | null;
  license_key?: string | null;
  purchased_count: number;
  purchased_at?: string | null;
  expires_at?: string | null;
  cost_total?: number | null;
  currency?: string;
  vendor_contact?: string | null;
  notes?: string | null;
}

export interface ComplianceSummary {
  total: number;
  compliant: number;
  over_used: number;
  expired: number;
  expiring_30: number;
  expiring_60: number;
  expiring_90: number;
  compliance_rate: number;
}

export interface LicenseInstallation {
  machine_uuid: string;
  hostname: string;
  os: string;
  software_name: string;
  software_version: string;
  last_seen_at: string | null;
}

export interface LicenseInstallationList {
  license_uuid: string;
  software_name: string;
  items: LicenseInstallation[];
  total: number;
}

// ── Module 7: overview dashboard ──────────────────────────────────────────────

export interface Overview {
  machines_total: number;
  agents_online: number;
  software_unique: number;
  vuln_critical: number;
  vuln_high: number;
  vuln_medium: number;
  vuln_low: number;
}

export type RiskColor = "green" | "yellow" | "orange" | "red";

export interface RiskScoreItem {
  machine_uuid: string;
  hostname: string;
  risk_score: number;
  color: RiskColor | string;
}

export interface RiskScoreList {
  items: RiskScoreItem[];
}

export interface VulnTrendPoint {
  date: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface VulnTrend {
  period_days: number;
  points: VulnTrendPoint[];
}

export interface RiskBreakdown {
  risk_score: number;
  color: RiskColor | string;
  unsigned: number;
  unauthorized: number;
  cve_critical: number;
  cve_high: number;
  cve_medium: number;
  cve_low: number;
}

// ── Module 8: reporting + export ──────────────────────────────────────────────

export type ReportType = "org_summary" | "machine_detail";
export type ReportFormat = "pdf" | "csv";
export type ReportStatus = "queued" | "running" | "completed" | "failed";

export interface Report {
  uuid: string;
  type: ReportType | string;
  format: ReportFormat | string;
  status: ReportStatus | string;
  machine_uuid: string | null;
  file_size: number | null;
  error_message: string | null;
  generated_by: string | null;
  schedule_uuid: string | null;
  created_at: string;
  completed_at: string | null;
  expires_at: string | null;
}

export interface ReportGenerateInput {
  type: ReportType;
  format: ReportFormat;
  machine_uuid?: string | null;
}

export interface ReportQueued {
  uuid: string;
  status: ReportStatus | string;
}

export interface ReportSchedule {
  uuid: string;
  name: string;
  type: ReportType | string;
  format: ReportFormat | string;
  machine_uuid: string | null;
  cron: string;
  recipients: string[];
  is_active: boolean;
  last_run_at: string | null;
  next_run_at: string | null;
  upcoming_runs: string[];
  created_at: string;
}

export interface ReportScheduleInput {
  name: string;
  type: ReportType;
  format: ReportFormat;
  machine_uuid?: string | null;
  cron: string;
  recipients?: string[];
  is_active?: boolean;
}
