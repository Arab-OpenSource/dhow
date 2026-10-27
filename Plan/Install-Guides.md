# خطة شرح التنصيب لكل الطرق

**الحالة:** معتمدة (المرحلتين أ وب)، وHelm مع M3 · **التاريخ:** 2026-10-27

## السياق
- `docs/install.md` بيشرح Docker Compose بس. الـ reverse proxy مذكور في مثال Caddy من سطرين.
- [ADR-0024](ADR/0024-install-methods.md) بيقول إن فيه أربع طرق رسمية: Compose وHelm وbinary + systemd، وحزمة air-gapped في Enterprise. وكل طريقة ليها اختبار في الـ CI ومسار ترقية موثّق.
- `docs/roadmap.md` حاطط Helm وbinary + systemd في M10 (public beta). يعني الخطة دي بتقدّمهم.
- **قيود السيرفر اللي أي شرح لازم يحترمها:**
  - `DHOW_PUBLIC_URL` لازم يكون `https://`، إلا لو `DHOW_DEV=true`.
  - الاتصال بقاعدة البيانات لازم يكون بـ `sslmode=require` أو أعلى، إلا لو `DHOW_DEV=true`.
  - أي طلب مش GET لازم يكون الـ `Origin` بتاعه هو `DHOW_PUBLIC_URL` بالظبط.
  - الـ cookie اسمه `__Host-dhow_session`، فهو `Secure` ومن غير `Domain`. يعني داو لازم ياخد الدومين كله أو subdomain، ومينفعش يتحط تحت path زي `/dhow`.
  - عنوان العميل بيتاخد من `X-Forwarded-For` بس لو الطلب جاي من `DHOW_TRUSTED_PROXIES`.

## الهيكل المقترح للوثائق
```
docs/install/
  README.md           اختيار الطريقة: جدول مقارنة + المتطلبات المشتركة
  compose.md          (نقل docs/install.md الحالي كما هو تقريبًا)
  reverse-proxy.md    nginx وCaddy وTraefik
  local.md            التجربة والتطوير على localhost
  binary-systemd.md   المرحلة ب (يدوي)
  installer.md        المرحلة ب (الـ installer التفاعلي)
  kubernetes.md       مع M3
  configuration.md    كل متغيرات DHOW_* في مكان واحد
  troubleshooting.md  الأخطاء الشائعة (Origin وTLS وproxy وcookie)
```
- `docs/install.md` بيفضل موجود كصفحة قصيرة بتحوّل لـ `docs/install/README.md` عشان الروابط القديمة.
- كل الوثائق بالإنجليزي (قاعدة `docs/`).

## المرحلة أ: وثائق بس (من غير تغيير في الكود)
1. **`reverse-proxy.md`:**
   - **إعداد كامل لكل proxy:** nginx (مع certbot)، وCaddy (TLS تلقائي)، وTraefik (labels لـ Compose).
   - **الحاجات المشتركة:** `proxy_set_header Host` و`X-Forwarded-For` و`X-Forwarded-Proto`، وإن الـ proxy مايشيلش `Origin` ولا يغيّره. وفيه كمان `client_max_body_size`، وtimeouts مناسبة للـ SSE اللي جاي في M3 (`proxy_buffering off` على مسار الـ logs)، و`DHOW_TRUSTED_PROXIES` بالـ CIDR الصح.
   - **فحص بعد الإعداد:** `curl` على `/api/v1/health/ready`، وتسجيل دخول من المتصفح.
   - **قبول:** كل إعداد اتجرّب فعلًا في Docker ورا شهادة self-signed، وتسجيل الدخول والإعداد الأول اشتغلوا من خلاله (Playwright). وفيه script في `deploy/proxy-check/` بيشغّل الثلاثة ويتحقق، ويتنادى من `make proxy-check` (محلي فقط في الأول).
2. **`local.md`:**
   - **طريقتين:** الأولى `make web && go run ./apps/server/cmd/dhow-server` مع Postgres 18 في Docker، والتانية الـ Compose stack على `http://127.0.0.1:8080`.
   - **التحذير:** تحذير واضح وبارز إن `DHOW_DEV=true` بيقفل حمايات ومينفعش في production، والسيرفر بيكتب تحذير في الـ log لما يقوم بيه (موجود في `App/Server.go`).
   - **قبول:** الخطوات اتنفّذت من الصفر في container نضيف، ونجحت.
3. **`configuration.md`:** جدول بكل متغير، والقيمة الافتراضية، وهل إجباري، ومثال. المصدر هو `apps/server/internal/Config/Env.go`، وفيه اختبار بيفشل لو متغير اتضاف في الكود ومش في الجدول.
4. **`troubleshooting.md`:**
   - `CSRF_FAILED` بسبب `Origin` مختلف.
   - الـ cookie مش بيتحفظ (http، أو path، أو دومين).
   - رفض `http://` في `DHOW_PUBLIC_URL`.
   - `sslmode`.
   - الـ IP اللي في سجل الأحداث هو IP الـ proxy.

## المرحلة ب: binary + systemd + installer تفاعلي (فيها كود، [ADR-0036](ADR/0036-interactive-host-installer.md))
1. **`release.yml`:**
   - binaries لـ `linux/amd64` و`linux/arm64`، كل واحد في `tar.gz`، والواجهة جوه الـ binary.
   - `SHA256SUMS`، وتوقيع cosign keyless بـ `sign-blob`، وSBOM لكل أرشيف، وattestation.
   - **قبول:** `cosign verify-blob` بيعدّي في الـ workflow نفسه.
2. **`Config`:**
   - قراية `DHOW_SECRET_KEY_FILE` و`DHOW_DATABASE_URL_FILE`.
   - لو الاتنين موجودين (المتغير والملف) يبقى خطأ، ولو الملف صلاحياته مفتوحة للكل يبقى خطأ برضه. والاختبارات table-driven.
3. **`deploy/systemd/dhow.service`:**
   - `DynamicUser=no` مع user `dhow` اللي الـ installer بيعمله.
   - `ProtectSystem=strict`، و`ProtectHome=yes`، و`NoNewPrivileges=yes`، و`PrivateTmp=yes`، و`CapabilityBoundingSet=` فاضي، و`RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`، و`SystemCallFilter=@system-service`، و`MemoryDenyWriteExecute=yes`.
   - الأسرار بـ `LoadCredential=`.
   - **قبول:** `systemd-analyze security` بيدّي «OK» أو أحسن.
4. **`deploy/install/get.sh` (bootstrap):**
   - بيعرف المعمارية، وبينزّل آخر release أو نسخة محددة بـ `--version`.
   - بيتحقق من `SHA256SUMS` وتوقيع cosign لو `cosign` موجود. ولو مش موجود بيتحقق بالـ checksum، وبيحذّر وبيقول إزاي تتحقق بإيدك. وبعدها بيشغّل `dhow-server install`.
   - لازم يشتغل بـ `bash -n` وshellcheck من غير تحذيرات.
5. **`dhow-server install` (Go):**
   - **أوامر فرعية:** `install` و`doctor` و`uninstall`، ومعاهم `--answers` و`--dry-run` و`--yes` و`--fix`.
   - **الاكتشاف:** التوزيعة من `/etc/os-release`، وsystemd، وDocker، وPostgres، وnginx وCaddy، والبورتات المشغولة، والـ firewall (ufw أو firewalld).
   - **الأسئلة:**
     - مسار التنصيب: من غير Docker أو Docker Compose.
     - الدومين والـ `DHOW_PUBLIC_URL`.
     - Postgres: ينصّب 18 من PGDG أو قاعدة موجودة (URL وفحص اتصال وإصدار).
     - الـ proxy: Caddy أو nginx مع certbot، أو proxy موجود (بيطبع الإعداد)، أو مفيش.
     - الـ firewall.
     - أي package ناقص بيسأل عليه قبل ما ينصّبه.
   - **قبل التنفيذ:** بيعرض الخطة كلها (كل خطوة، وأي ملف هيتكتب، وأي package هيتنصّب)، وبعدها بيطلب تأكيد واحد.
   - **التنفيذ:** كل خطوة idempotent وليها rollback لو فشلت قبل ما تخلص. والأسرار بتتولّد على السيرفر وعمرها ما بتتطبع. وفي الآخر بيطبع رابط الإعداد الأول.
   - **`doctor`:** بيفحص الخدمة والصلاحيات والأسرار والـ TLS والـ proxy (`Host` و`Origin` و`X-Forwarded-*`) والـ migrations، ولكل مشكلة إصلاح مقترح.
   - **`uninstall`:** بيشيل الخدمة والملفات، والبيانات وقاعدة البيانات بتفضل إلا بـ `--purge` وتأكيد مكتوب.
   - **هيكل الكود:** في `apps/server/internal/Installer/` بنفس قواعد الطبقات، والـ steps ورا interface عشان الاختبارات تشتغل على نظام وهمي.
6. **CI:**
   - matrix (Debian 12 وUbuntu 24.04 وRocky 9 وFedora) بيشغّل `install --answers` في container فيه systemd، وبيتحقق من `/health/ready` وتسجيل الدخول.
   - وبعدها `doctor` لازم يطلع نضيف، و`uninstall` لازم يسيب السيرفر نضيف.
   - واختبار للمسار التفاعلي بـ pty.
7. **الوثائق:** `docs/install/binary-systemd.md` (يدوي) و`docs/install/installer.md` (الـ installer وملف الإجابات وdoctor)، والترقية والنسخ الاحتياطي وإزالة التنصيب.

## المرحلة ج: Kubernetes / Helm (اتأجلت لـ M3)
- هتتعمل مع الـ runners عشان الـ chart يتصمم مرة واحدة (server وrunners وNetworkPolicy بينهم).
- التصميم المبدئي:
  - Postgres خارجي إجباري، و`existingSecret`، وrestricted Pod Security.
  - CI على `kind`.
- اتسجّلت في `docs/roadmap.md` تحت M3.

## الترتيب والـ PRs
1. **PR 1:** المرحلة أ (وثائق وفحص الـ proxies)، ومعاها الخطة دي وADR-0036.
2. **PR 2:** المرحلة ب (release binaries و`Config` وsystemd والـ installer والـ CI).

كل PR بيعدّي `make check` وCI أخضر. وتقديم الـ binary والـ installer من M10، وتأجيل Helm لـ M3، الاتنين بيتسجّلوا في `docs/roadmap.md`.

## Edge cases
- داو تحت path فرعي (`example.com/dhow`) مش مدعوم بسبب الـ `__Host-` cookie. ده لازم يبقى مكتوب صراحة في `reverse-proxy.md`.
- proxy قدّام proxy (Cloudflare ثم nginx): `DHOW_TRUSTED_PROXIES` لازم يشمل الاتنين، وإلا الـ IP هيبقى غلط والـ throttle هيقفل الناس كلها مع بعض.
- WebSocket وSSE في M3: الإعدادات هتتجهّز من دلوقتي عشان المستخدم مايرجعش يعدّل.
- localhost على المتصفح: `http://localhost` بيعتبر secure context، فالـ `Secure` cookie بيشتغل عليه في Chrome وFirefox، لكن مش على IP زي `http://192.168.x.x`. ولازم ده يتكتب.
