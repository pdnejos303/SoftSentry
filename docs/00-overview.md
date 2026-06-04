# 00 — Product Overview

## Problem

IT/Security ใน องค์กรขนาดกลาง-ใหญ่ ต้องตอบคำถามทุกวัน:

- มี software อะไรติดตั้งบนเครื่องพนักงานบ้าง?
- ใครติดตั้ง software ที่ไม่ได้รับอนุญาต?
- มี software ที่ unsigned/มี malware risk หรือเปล่า?
- Software ที่ใช้อยู่มี vulnerability (CVE) ไหม?
- License compliance: ซื้อพอใช้ไหม? หมดอายุเมื่อไหร่?

เครื่องมือสำเร็จรูป (Lansweeper, Microsoft Intune, Tanium) **แพง** + **ไม่ flex** + **lock-in**

## Solution

**SoftSentry** — self-hosted, open-stack, focused

ติดตั้ง agent บนเครื่อง endpoint → backend รวบรวม + วิเคราะห์ → dashboard ตอบทุกคำถามด้านบนได้ในที่เดียว

## Target Users

| Persona | งาน | สิ่งที่ต้องการ |
|---------|------|----------------|
| **IT Admin** | จัดการ inventory + license | รายการเครื่อง, search, history |
| **Security Engineer** | ตามหา risk + CVE | Risk score, alert feed, drill-down |
| **Compliance Officer** | License + audit | License status, expiry, audit log |
| **Manager** | Reporting | PDF report ส่งให้ผู้บริหาร |

## Scope

### In scope (v1)
- Windows + macOS endpoints
- 9 modules ตามที่ระบุใน [ROADMAP](../ROADMAP.md)
- Single tenant (1 องค์กร / 1 deployment)
- ไทย + EN UI

### Out of scope (v1)
- Linux endpoints
- Mobile (iOS/Android)
- Multi-tenant SaaS
- SIEM integration (Splunk/Sentinel)
- EDR features (process monitor, file integrity)
- Patch management (เราแค่ detect, ไม่ patch)

### Out of scope แต่จะคิดในอนาคต (v2+)
- Linux agent
- SAML/SCIM SSO
- Slack/Teams notification
- REST API public (สำหรับ integrate ระบบอื่น)
- Active Directory integration

## Non-functional Requirements

| ด้าน | เป้าหมาย |
|------|----------|
| Scale | 1,000 endpoints / 1 backend instance |
| Scan frequency | Default 6 ชม., configurable 1ชม.-24ชม. |
| Agent footprint | RAM <= 50MB idle, CPU <= 5% ขณะ scan |
| Dashboard load | <= 2s ที่ 1,000 machines |
| Uptime | 99% (single deployment, no HA ใน v1) |
| Data retention | History 1 ปี |

## Success Metrics

- IT admin ตอบคำถาม "เครื่อง A มี Photoshop หรือไม่?" ภายใน 10 วินาที
- เจอ unauthorized software ภายใน 1 scan cycle
- License audit ใช้เวลา < 1 ชม. (เทียบกับ manual หลายวัน)
- 0 false positive ใน critical CVE alert (severity Critical แล้วต้องจริง)

## Constraints

- **Privacy**: agent ห้ามส่งข้อมูลส่วนตัวของ user (browser history, documents, ฯลฯ) — เฉพาะ software metadata
- **Network**: agent ทำงานได้แม้ไม่มี internet ออกเน็ตได้แค่ backend (no external call)
- **Self-hosted**: ทุก dependency ต้อง self-hostable (รวมถึง CVE database mirror)
