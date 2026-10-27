# ADR-0048: تغليف الـ runner: binary وsystemd ووضع `runner` في الـ installer، وHelm chart واحد، وpod لكل job على Kubernetes

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيكمّل:** [ADR-0024](0024-install-methods.md) و[ADR-0036](0036-interactive-host-installer.md) و[ADR-0043](0043-job-sandbox-rootless-podman.md).

## السياق
- ADR-0036 أجّل Helm لـ M3 عشان يتصمم مرة واحدة والـ runners جواه، وقال إن الـ installer هيبقى subcommand في Go.
- على Kubernetes، الـ runner محتاج يا إما sandbox من غير privileges، يا إما node pool لوحده. Podman rootless جوه pod محتاج user namespaces متداخلة و`/dev/fuse` وseccomp وAppArmor مفتوحين أو `privileged`، وده بيكسر Pod Security «restricted».
- المطلوب في الـ chart: Postgres خارجي، و`existingSecret`، وrestricted Pod Security، وNetworkPolicy بين السيرفر والـ runners.

## القرار

### الـ binary
- **`dhow-runner`:** static (`CGO_ENABLED=0`) لـ `linux/amd64` و`linux/arm64`. بيتنشر في نفس الـ release مع `SHA256SUMS` وتوقيع cosign وSBOM، زي `dhow-server`.
- **الأوامر:**
  - `run` (الخدمة).
  - `register`.
  - `install` و`doctor` و`uninstall`.
  - `rotate-keys`.
  - أوامر داخلية للـ sandbox: `agent` و`connect` و`mask`.
- **هيكل الكود:** `apps/runner/cmd/dhow-runner` و`apps/runner/internal/<Name>` بـ PascalCase زي السيرفر:
  - `App` و`Config`.
  - `Client` (Connect والمصادقة).
  - `Identity` و`Journal`.
  - `Executor` و`Sandbox/{Podman,Kubernetes}`.
  - `Agent` و`Egress` و`Masking` و`Installer`.

  الـ hook بتاع الهيكل بيتوسّع ليشمل `apps/runner/internal`.

### التنصيب على الـ host (وضع `runner` في الـ installer)
- **الـ bootstrap:** `deploy/install/get.sh --component runner` بينزّل `dhow-runner` ويتحقق منه بنفس طريقة السيرفر، وبعدين بيشغّل `dhow-runner install`.
- **الـ engine مشترك:** الـ installer engine (الخطوات والخطة والـ dry-run وملف الإجابات والـ rollback) بيتنقل لـ `internal/installer` في جذر الـ repo، عشان السيرفر والـ runner يستخدموه. لو المرحلة ب في `Plan/Install-Guides.md` لسه ماتعملتش، M3.10 هو اللي بيبني الـ engine هناك.
- **الخطوات (كل واحدة بتتعرض في الخطة وبتتسأل):**
  1. **الفحص:**
     - kernel فيه cgroup v2 وuser namespaces وseccomp. وعلى Ubuntu، `kernel.apparmor_restrict_unprivileged_userns`.
     - Podman 5.4+ وcrun وuidmap.
     - systemd و`systemd-creds`.
  2. تنصيب الناقص من مستودعات التوزيعة، بعد السؤال.
  3. **user النظام `dhow-runner`:** من غير shell، والـ home هو `/var/lib/dhow-runner`، ونطاق subuid وsubgid حجمه 1048576 (16 job متزامن × 65536).
  4. **`dhow-runner.service`:**
     - `User=dhow-runner` و`Delegate=yes` و`MemorySwapMax=0` و`LimitCORE=0` و`KillMode=control-group`.
     - `ProtectSystem=strict` و`ReadWritePaths=/var/lib/dhow-runner`، و`ProtectHome=yes` و`PrivateTmp=yes`.
     - `ProtectKernelModules=yes` و`ProtectClock=yes` و`RestrictRealtime=yes` و`LockPersonality=yes`.
     - التقوية الكاملة بتتحدد بالتجربة، لأن `newuidmap` محتاج `CAP_SETUID` و`CAP_SETGID` في الـ bounding set ومن غير `NoNewPrivileges`.
  5. **التسجيل:** التوكن من ملف أو prompt مخفي، وبعدين تشفير ملف الهوية بـ `systemd-creds` لو متاح.
  6. **gVisor (اختياري):** تنزيل `runsc` بالنسخة المثبّتة والتحقق من SHA-512.
  7. **التشغيل، وبعده `doctor`:** بيشغّل job تجريبي صغير محلي (`hello` بـ digest) ويتأكد من الحدود.
- **ملف الإجابات وإعادة التشغيل وغيرها:** `--answers` و`--dry-run` و`doctor --fix` و`uninstall --purge` بنفس سلوك ADR-0036. الـ uninstall بيلغي تسجيل الـ runner في السيرفر لو يقدر.
- **الـ runner والسيرفر على نفس الجهاز:** مسموح للـ VPS الصغير، مع تحذير في الخطة. الـ user منفصل، ومالوش وصول لـ credentials السيرفر.
- **Docker:** لو موجود على الجهاز، الـ runner مابيلمسوش ومابيستخدمهوش.

### Kubernetes: pod لكل job (driver اسمه `kubernetes`)
- **الـ runner:**
  - pod عادي بـ restricted Pod Security، ومابيشغّلش حاويات بنفسه.
  - لكل attempt بيعمل Pod في namespace مخصص للـ jobs (`dhow-jobs`)، بـ RBAC محصور في الـ namespace ده: `pods` (create وget وlist وwatch وdelete) و`events` (get وlist) بس. مفيش `secrets` ولا `pods/exec`.
  - بيكلّم الـ Kubernetes API بـ HTTPS من الـ standard library وstructs صغيرة، مش `client-go` (اعتمادية ضخمة لعمليات pods بس).
- **pod الـ job:**
  - **الـ namespace:** `pod-security.kubernetes.io/enforce: restricted`.
  - **الـ security context:** `runAsNonRoot` و`allowPrivilegeEscalation: false` و`capabilities.drop: [ALL]` و`seccompProfile: RuntimeDefault` (أو ملفنا بـ `Localhost` لو المشغّل ركّبه على الـ nodes) و`readOnlyRootFilesystem: true`.
  - **العزل:** `hostUsers: false` لو الـ cluster بيدعمه (GA من Kubernetes 1.36)، و`automountServiceAccountToken: false` و`enableServiceLinks: false`. و`dnsPolicy: None` مع nameserver على `127.0.0.1`، يعني مفيش DNS.
  - **الحدود:**
    - الـ requests بتساوي الـ limits للـ CPU والذاكرة.
    - `/workspace` كـ `emptyDir` بـ `medium: Memory` و`sizeLimit`.
    - `ephemeral-storage` limit، و`activeDeadlineSeconds` كحماية أخيرة، و`restartPolicy: Never`.
  - **الـ agent:** init container من image الـ runner (بـ digest) بيحط `dhow-runner` في `emptyDir`، والحاوية الأساسية هي image الـ job والـ entrypoint بتاعها الـ agent.
  - **قناة التحكم:**
    - الـ agent بيفتح TLS لـ pod الـ runner على port 7443، ومعاه توكن لمرة واحدة عمره دقيقتين في env الـ pod، وبيثبّت بصمة شهادة الـ runner (شهادة مؤقتة لكل process).
    - الأسرار بتيجي على القناة دي بس، عمرها ما بتبقى Kubernetes Secret ولا في spec الـ pod. والتوكن بيتلغي أول ما يتستخدم.
  - **الـ egress:** على `127.0.0.1:3128` جوه الـ pod، وبيوصل لـ proxy الـ runner على port 3128.
  - **مستوى `gvisor`:** `runtimeClassName` من الـ values (مثلًا `gvisor`)، والـ runner بيعلن `gvisor` بس لو الـ RuntimeClass موجود.
- **node pool لوحده: موصى بيه ومش إجباري.** الـ chart فيه `jobs.nodeSelector` و`jobs.tolerations` (مثلًا taint `dhow.dev/jobs=true:NoSchedule`)، والوثائق بتقول صراحة: كود مش موثوق = node pool لوحده وgVisor.
- **الموت والتنظيف:**
  - لو pod الـ runner مات، قناة التحكم بتتقفل والـ agent بيخرج، فالـ pod بيخلص.
  - والـ runner لما يقوم بيمسح أي pod عليه `dhow.dev/runner=<id>`.
  - و`activeDeadlineSeconds` بيقفل أي حاجة فلتت.

### Helm chart (`deploy/helm/dhow`)
- **chart واحد** فيه `server` و`runner` و`jobs`، وأي واحد يتقفل (`server.enabled=false` لـ runners في cluster تاني).
- **السيرفر:**
  - Deployment بـ replica واحدة. أكتر من replica محتاج S3 للـ blob store (M4)، والـ HA ميزة Business.
  - Service فيها `http` 8080، و`runner` 8443 بـ TLS من `runnerPlane.tls.existingSecret` أو cert-manager (`runnerPlane.tls.certManager.issuerRef`). ولو مفيش ولا واحد منهم، الـ chart بيفشل (`fail`).
  - Ingress اختياري.
- **قاعدة البيانات:** خارجية وإجبارية: `database.existingSecret` (مفتاح `url`). مفيش Postgres جوه الـ chart.
- **الأسرار:**
  - `secretKey.existingSecret` إجباري.
  - الـ chart بيرفض أي سر مكتوب في الـ values نفسها (`fail`)، والقبول بـ `existingSecret` بس.
  - توكن تسجيل الـ runners في `runner.registration.existingSecret`، والـ `MaxUses` بتاعه قد عدد الـ replicas.
- **الـ runner:** StatefulSet بـ PVC لكل replica (ملف الهوية)، وService headless لقناة التحكم والـ proxy.
- **كل الـ pods:** restricted Pod Security، والـ images بـ digest، و`readOnlyRootFilesystem`.
- **NetworkPolicies (deny افتراضي في الـ namespaces التلاتة):**
  - **السيرفر:** يستقبل على 8080 من الـ ingress controller بس (namespace selector في الـ values)، وعلى 8443 من pods الـ runner بس. ويخرج لقاعدة البيانات (CIDR أو selector من الـ values) والـ DNS بس.
  - **الـ runner:** يخرج للسيرفر على 8443، والـ Kubernetes API، والـ DNS، وللإنترنت (للـ proxy) بس. ويستقبل على 7443 و3128 من pods الـ jobs بس.
  - **pods الـ jobs:** تخرج لـ pods الـ runner على 7443 و3128 بس، ومابتستقبلش أي حاجة.
- **`jobs` namespace:** فيه `ResourceQuota` و`LimitRange` من الـ values.

## البدائل اللي اتدرست
- **Podman rootless جوه pod الـ runner:** محتاج `privileged` أو `procMount: Unmasked` وseccomp `Unconfined`، وده بيكسر restricted وbaseline.
- **sysbox أو Kata Containers:** عزل كويس، لكنهم محتاجين تجهيز للـ nodes، ومش متاحين في أغلب الـ clusters المُدارة. ممكن يبقوا RuntimeClass تاني بعدين بنفس الـ driver.
- **`client-go`:** الطريقة المعتادة، لكنه بيجيب مئات الـ packages عشان عمليات pods بس.
- **chart لكل مكوّن:** إعدادات مكرّرة (الـ TLS والـ NetworkPolicy بين الاتنين). والـ chart الواحد بـ `enabled` بيغطي الحالتين.
- **Deployment للـ runners بتوكن متعدد الاستخدام في كل إقلاع:** كل restart كان هيعمل runner جديد، والتوكن لازم يفضل صالح على طول.

## العواقب
- على Kubernetes، الحد على القرص (`ephemeral-storage`) بيتطبّق بالـ eviction (حد مرن). والـ `/workspace` في الذاكرة هو الحد الصارم.
- الـ NetworkPolicy محتاجة CNI بيطبّقها. الـ chart بيكتب تحذير في `NOTES.txt`، و`doctor` بتاع الـ runner بيجرّب اتصال ممنوع ولازم يفشل.
- **الـ CI:** kind (فيه kindnet، وبيطبّق NetworkPolicy) وHelm 3 و4 وkubeconform. وgVisor على Kubernetes بيتجرّب على VM بس.
- `docs/install/kubernetes.md` و`docs/install/runner.md` جداد.
