# ADR-0057: تنفيذ الـ egress proxy في M3.7

- **الحالة:** مقبول. فيه 3 قرارات خدها Sabry، والباقي مفوَّض.
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0044](0044-job-network-egress-proxy.md) و[ADR-0055](0055-podman-driver-and-agent-implementation.md) و[ADR-0056](0056-sandbox-hardening-after-the-m3-6-review.md).

## السياق
ADR-0044 قرّر إن الحاوية تفضل `--network=none`، وإن الخروج يعدّي من proxy في الـ runner بقايمة hosts. M3.7 هو أول تنفيذ للقرار ده، والتنفيذ ساب أسئلة لازم تتقفل:
- الـ parser بيقبل إيه بالظبط؟
- مين بيشغّل الـ proxy، وفي أنهي طبقة؟
- إمتى الـ agent يفتح `3128`؟
- الـ helper `connect` بيتركّب إزاي؟
- الـ operator range بيفتح إيه بالظبط؟

## القرار

### القايمة (`internal/jobspec/egress.go`)
- **`ParseEgress`:** السيرفر والـ runner بيستخدموه هو نفسه.
  - كل عنصر يا `host`، يا `host:port`، يا `*.domain`.
  - الـ port رقم من 1 لـ 65535، من غير أصفار في الأول.
- **الأسماء (قرار Sabry):** حروف صغيرة ASCII بس، وأي script تاني بالـ punycode (`xn--…`). الـ Unicode مرفوض برسالة واضحة. كده مفيش اعتمادية IDNA، ومفيش مشكلة الحروف اللي شبه بعض.
- **حاجات مرفوضة لأنها ممكن تتفهم على إنها IP:**
  - أي IP صريح.
  - أي اسم آخر label فيه أرقام بس (`2130706433`) أو بيبدأ بـ `0x`، لأن الـ resolvers بتحسبهم IPv4.
- **`*.domain`:** بيطابق أي subdomain مهما كان عمقه، ومابيطابقش `domain` نفسه. والـ wildcard بيتكتب في أول الاسم بس.
- **من غير port:** 443 مسموح لـ `CONNECT` بس، و80 مسموح لـ `http://` العادي بس.
- **`NormalizeHost`:** بيحوّل host الطلب لحروف صغيرة وبيشيل نقطة واحدة من الآخر، وبعدين بيطبّق عليه نفس القواعد.

### طبقة `Egress` في الـ runner
- طبقة جديدة في `apps/runner/internal/Egress`، ومابتستوردش أي طبقة تانية من الـ runner. الـ `Executor` هو اللي بيشغّلها، والـ hook (`runner_rules.py`) بيفرض ده.
- الـ proxy بـ standard library بس، ومن غير أي MITM.
- **الطلبات المقبولة:**
  - `CONNECT host:port`.
  - `GET http://…` بصيغة absolute-form. قبل ما الطلب يتبعت، بيتشال منه أي header من نوع hop-by-hop، و`Proxy-Authorization`، وأي header مذكور في `Connection`، وبيتبعت بـ `Connection: close`.
  - `Expect: 100-continue` بيتشال، والـ proxy بيرد على العميل بنفسه بـ `100 Continue`.
  - أي URL بـ `https://` من غير `CONNECT` بيترفض، وكمان الطلب بالـ origin-form.
- **الحدود:**
  - رأس الطلب 16 KiB بالكتير، ولازم يوصل في 30 ثانية.
  - الـ DNS والاتصال 10 ثواني لكل واحد.
  - 256 اتصال في نفس الوقت لكل job، والزيادة بتاخد `503`.
  - الاتصال بيتقفل بعد 5 دقايق من غير نشاط في الاتجاهين مع بعض. يعني download طويل ماحدش بيرفع فيه حاجة مش هيتقفل.
- **الاسم والعنوان:** الاسم بيتحل في كل اتصال لوحده، وكل عنوان بيرجع بيتفحص. الاتصال بيروح للعنوان اللي اتفحص بس، فالـ DNS rebinding مابيعدّيش. ولو كل العناوين ممنوعة، الطلب بيترفض بـ `403`.
- **النطاقات الممنوعة:**
  - كل اللي في ADR-0044.
  - `64:ff9b::/96`، وده NAT64 ممكن يلفّ لعنوان IPv4 داخلي.
  - `2001:db8::/32`.
  - `169.254.170.2`، وده metadata بتاع ECS.
  - عناوين كل الـ interfaces على جهاز الـ runner، وبتتقري لما الـ attempt يبدأ.
- **`ALLOW_PRIVATE_CIDRS` (قرار Sabry):** أي نطاق المشغّل يكتبه بيفتح كل حاجة جوّاه، ومنها عناوين جهاز الـ runner نفسه. ده لازم عشان registry داخلي على نفس الجهاز، وعشان اختبار الـ CI.
  - الـ metadata بتفضل ممنوعة دايمًا، حتى لو النطاق بيغطيها.
  - الـ job مابيوصلش للنطاقات دي غير باسم مكتوب في قايمته. أي IP صريح بيترفض دايمًا.
- **السجل:**
  - كل قرار بيطلع سطر `dhow: egress allowed host:port` أو `dhow: egress denied host:port (السبب)` في log الـ job.
  - نفس القرار بيتكتب مرة واحدة في الدقيقة بس.
  - أي host مش صالح بيتكتب بين علامات تنصيص وبـ ASCII، فمايقدرش يحقن escape codes في الـ log.

### الـ agent والـ Podman driver
- لو الـ job عنده قايمة egress، الـ driver بيفتح `egress.sock` في مجلد التحكم بتاع الـ attempt بـ `0600`، جنب `control.sock`، ونفس `U=true` بيدّيه للـ userns. ولو مفيش قايمة، مفيش socket خالص.
- الـ agent أول ما يقوم بيدوّر على `/run/dhow/control/egress.sock`.
  - **لو موجود:** بيفتح `127.0.0.1:3128`، وكمان `[::1]:3128` لو الـ IPv6 موجود، وبيوصّل كل اتصال بالـ socket كما هو، بحد 256.
  - **لو مش موجود:** مفيش أي listener، والـ probe بيتأكد إن البورت مقفول.
- الـ job يقدر يكلّم `egress.sock` مباشرة من غير الـ agent. ده مقبول، لأن السياسة كلها مطبّقة على طرف الـ runner، والـ forwarder مجرد طريقة سهلة للبرامج اللي بتعرف `HTTP_PROXY`.
- **`/run/dhow/bin/connect`:** هو binary الـ runner نفسه، متركّب read-only في كل حاوية، وبيتعرف من `argv[0]`. بيعمل `CONNECT` على `egress.sock` نفسه، وبيوصّل الـ stdin والـ stdout. و`verify` بيقبل الـ mount ده بس لو مصدره binary الـ agent.

### الـ executor
- الـ executor بيشغّل الـ proxy لكل attempt عنده قايمة، بعد ما الـ sandbox يتجهّز وقبل الـ start. والـ proxy بيقف لما الـ attempt يخلص أو لما الـ sandbox يتمسح. ولو الـ proxy ماقامش، الـ attempt بيفشل بـ `SANDBOX_FAILED`.
- `HTTPS_PROXY` و`HTTP_PROXY` و`ALL_PROXY` (بالحروف الكبيرة والصغيرة) بتبقى `http://127.0.0.1:3128`، و`NO_PROXY` بيبقى `localhost,127.0.0.1,::1`. وبتتكتب بعد env الـ job والأسرار، فالـ job مايقدرش يغيّرها.

### الـ fuzz
- `FuzzAllowlist` في `internal/jobspec`، لأن الـ parser بقى هناك عشان السيرفر والـ runner يستخدموه، مش في `Egress` زي ما كان في الخطة.
- `FuzzProxyRequest` في `Egress`: بايتات عشوائية داخلة على الـ handler.

## العواقب
- الـ jobs اللي برامجها بتعرف تتعامل مع proxy (npm وpip وgo وcurl وgit على https وssh بـ `ProxyCommand`) بتوصل للـ hosts اللي في قايمتها وبس.
- **مش مدعوم:**
  - البرامج اللي مابتعرفش proxy.
  - UDP وQUIC.
  - `https://` بصيغة absolute-form.
- **IDNA:** الأسماء بالعربي لازم تتكتب punycode في القايمة. لو ده بقى مزعج، `golang.org/x/net/idna` ممكن يتضاف بعدين بـ ADR جديد.
- كل اتصال بيعدّي على الـ runner، فاستهلاك الـ CPU والذاكرة عليه بيزيد مع jobs كتير الاتصالات. الحد 256 اتصال لكل job بيحمي الجهاز.
