# خطة M1: الصورة والـ compose والتوقيع (نهاية M1)

**الحالة:** متوافق عليها («نقفل M1»، 2026-10-27) · **الفرع:** `m1/deploy`

## الهدف
شرط خلاص M1 في `docs/roadmap.md`: `docker compose up` بيقوّم نسخة داو فاضية وآمنة. ومعاها SBOM وتوقيع الإصدارات في الـ CI. المرجع: `docs/security-model.md` (§ Supply chain)، و[ADR-0024](ADR/0024-install-methods.md).

## المهام

### 1. فحص الصحة من جوه الـ binary
- `dhow-server healthcheck` بيطلب `GET /api/v1/health/ready` على العنوان المحلي، وبيرجع 0 أو 1. الصورة distroless ومفيهاش shell ولا curl، فالـ healthcheck لازم يكون في الـ binary نفسه.
- رفض الأسرار الوهمية: `DHOW_SECRET_KEY` لو كانت قيمة معروفة (`changeme` أو `secret` أو `example` أو كلها حرف واحد متكرر) بتوقف الإقلاع (Fail closed)، ومعاها اختبار.

### 2. الصورة (`deploy/docker/Dockerfile`)
- multi-stage:
  1. Node 24 وpnpm بيبنوا الواجهة.
  2. Go 1.27.3 بيبني `dhow-server` والواجهة جواه (`CGO_ENABLED=0` و`-trimpath` و`-buildid=`)، فالبناء قابل للتكرار.
  3. `gcr.io/distroless/static-debian12:nonroot`.
- كل base image متثبّتة بالـ digest.
- `USER 65532:65532`، ومفيش shell ولا package manager.
- `HEALTHCHECK` بيشغّل `dhow-server healthcheck`.
- `.dockerignore` بيستبعد `node_modules` و`dist` و`.git` و`.claude` و`bin`.

### 3. الـ compose (`deploy/compose/`)
- `compose.yaml`: `postgres:18` و`dhow` بالـ digest.
  - Postgres على شبكة داخلية بس، ومن غير port على الـ host.
  - `dhow` بيسمع على `127.0.0.1:8080` افتراضيًا.
  - إعدادات الحاويات:
    - `read_only: true`، و`tmpfs: /tmp`.
    - `cap_drop: [ALL]`، و`security_opt: no-new-privileges:true`.
    - حدود موارد (CPU وmemory وPIDs)، و`restart: unless-stopped`.
  - الـ healthchecks: `dhow` بيستنى Postgres يبقى healthy.
- `init.sh`:
  - بيولّد `.env` بأسرار قوية: `DHOW_SECRET_KEY` من 32 byte عشوائي، وكلمة سر Postgres.
  - صلاحيات الملف `0600`، ومش بيكتب فوق ملف موجود.
- `.env.example` من غير قيم.
- `README` الاستخدام مكانه `docs/install.md` (بالإنجليزي)، مش جوه `deploy/`.

### 4. الـ Makefile
- `make image` و`make compose-up` و`make compose-down` و`make smoke` (بيقوّم الـ compose ويفحصه ويقفله).

### 5. الـ CI
- **job `image` على كل PR:**
  - بيبني الصورة بـ buildx.
  - بيشغّل `make smoke`، اللي بيقوّم compose حقيقي ويتأكد من `/api/v1/health/ready` و`/api/v1/meta` و`/` والـ headers الأمنية.
  - فحص ثغرات للصورة (Trivy أو Grype)، ويفشل على critical أو high عندها إصلاح.
  - SBOM بصيغة SPDX كـ artifact.
- **workflow `release` على tag `v*`:**
  - صورة multi-arch (amd64 وarm64) على `ghcr.io/arab-opensource/dhow`.
  - توقيع keyless بـ cosign (OIDC من GitHub).
  - SBOM وSLSA provenance كـ attestations.
  - `permissions` بأقل حاجة ممكنة.
- كل الـ actions متثبّتة بالـ SHA. والـ release مش هيتجرّب فعليًا لحد أول tag، فالتحقق منه بـ actionlint بس.

### 6. الوثائق
- `docs/install.md`: التثبيت بالـ compose في 3 أوامر، والترقية، والنسخ الاحتياطي لـ Postgres، والتحقق من التوقيع بـ `cosign verify`.
- سطر في `README.md` بيشاور عليه.

## التحقق قبل الـ push
- `make check` و`make smoke` على Docker حقيقي.
- `docker run --rm --read-only` من غير env بيفشل وبيقول الإعداد الناقص.
- `docker inspect` بيأكد `User` 65532، و`docker run ... sh` بيفشل لأن مفيش shell.
- الـ image scan من غير critical أو high.

## مفتوح
- اسم الـ registry (`ghcr.io/arab-opensource/dhow`) محتاج تأكيد من Sabry وقت أول release.
