# ADR-0044: شبكة الـ job: `none` افتراضيًا، والـ egress من خلال proxy في الـ runner بقايمة hosts مسموحة

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيكمّل:** [ADR-0007](0007-job-isolation.md) و[ADR-0043](0043-job-sandbox-rootless-podman.md).

## السياق
- `docs/pipeline-spec.md` فيه `network.egress: [registry.npmjs.org]`، والفاضي معناه مفيش أي خروج.
- الـ job لازم ما يوصلش للشبكة الداخلية: الـ host نفسه، والـ metadata endpoints (`169.254.169.254`)، وقواعد البيانات الداخلية، والـ runners التانية.
- الـ allowlist بأسماء hosts، مش بـ IPs. الـ CDNs بتغيّر الـ IPs كل شوية، فـ allowlist بالـ IP يا إما هيقفل حاجات صح، يا إما هيفتح أكتر من اللازم.
- المطروح:
  - netns بـ slirp4netns أو pasta مع proxy بيفلتر.
  - شبكة داخلية في Podman (netavark) فيها container للـ proxy.
  - nftables جوه الـ rootless netns.

## القرار

### من غير egress
- `--network=none`: الحاوية فيها loopback بس، ومفيش DNS، ولا route لأي حتة.

### مع egress
- الحاوية برضه `--network=none`.
- الـ agent (PID 1) بيفتح listener على `127.0.0.1:3128` جوه الـ netns بتاع الحاوية. الـ loopback موجود حتى مع `none`.
- كل اتصال على الـ listener ده بيعدّي على unix socket خاص بالـ attempt (`egress.sock` في مجلد التحكم)، وعلى الطرف التاني الـ runner على الـ host شغّال كـ proxy بالقايمة بتاعة الـ job ده بس.
- الـ runner بيحط `HTTPS_PROXY` و`HTTP_PROXY` و`ALL_PROXY` (وبالحروف الصغيرة كمان) على `http://127.0.0.1:3128`، و`NO_PROXY=localhost,127.0.0.1`.
- **مفيش DNS جوه الحاوية.** الـ proxy هو اللي بيحل الأسماء، فمفيش قناة تسريب بالـ DNS.
- **الـ proxy (Go، standard library بس):**
  - بيدعم `CONNECT host:port` (HTTPS وأي TCP)، وطلبات HTTP العادية بصيغة absolute-form (عشان mirrors زي apt على `http://`).
  - من غير MITM ومن غير فك TLS.
  - الـ host بيتطابق مع القايمة على الاسم بعد التطبيع (IDNA وحروف صغيرة ومن غير نقطة في الآخر).
  - بعد كده بيحل الاسم، وبيرفض لو العنوان في نطاق ممنوع، وبيتصل بالعنوان اللي اتحل نفسه، مش بالاسم تاني. فـ DNS rebinding ما يعدّيش.
- **النطاقات الممنوعة دايمًا للـ job:**
  - IPv4: `0.0.0.0/8` و`10.0.0.0/8` و`100.64.0.0/10` و`127.0.0.0/8` و`169.254.0.0/16` و`172.16.0.0/12` و`192.0.0.0/24` و`192.168.0.0/16` و`198.18.0.0/15` و`224.0.0.0/4` و`240.0.0.0/4`.
  - IPv6: `::1/128` و`::/128` و`fc00::/7` و`fe80::/10` و`ff00::/8` و`fd00:ec2::254`. وأي IPv4-mapped (`::ffff:0:0/96`) بيتفحص على النطاق الـ IPv4 بتاعه.
  - وكل عناوين الـ interfaces بتاعة جهاز الـ runner نفسه.
  - المشغّل بس يقدر يفتح نطاق خاص (registry داخلي مثلًا) بـ `egress.allow_private_cidrs` في إعداد الـ runner. الـ job مايقدرش، حتى لو كتب IP صريح.

### صيغة القايمة في الـ job
- `host` أو `host:port` أو `*.example.com` (أي subdomain، من غير الـ apex نفسه).
- **من غير port:** المسموح 443، و80 للـ HTTP العادي.
- IPs صريحة مرفوضة في قايمة الـ job، والحد 64 عنصر.
- القايمة بتتحقق في السيرفر وقت إنشاء الـ job، وتاني في الـ runner. والـ runner بيعتبر قايمة السيرفر هي السقف.

### الشفافية والحدود
- كل قرار من الـ proxy (مسموح أو مرفوض، والـ host والـ port) بيظهر كسطر في log الـ job على stream اسمه `system`. فاللي كاتب الـ pipeline يعرف ليه الاتصال اترفض.
- الحد 256 اتصال متزامن لكل job، وأي اتصال من غير نشاط 5 دقايق بيتقفل.
- **الـ helper `/run/dhow/bin/connect host port`:** هو الـ agent نفسه في وضع تاني، بيعمل CONNECT ويوصّل stdin وstdout. ده بيخلّي `ssh -o ProxyCommand` وأي أداة شبهه تعدّي من نفس الـ proxy (ده اللي هتحتاجه deploy plugins في M6).
- **مش مدعوم في M3:**
  - البرامج اللي مابتعرفش proxy (زي drivers قواعد بيانات كتير) مش هتقدر توصل.
  - UDP وQUIC مش مدعومين.
  - ده مكتوب في `docs/pipeline-spec.md`. وأي egress شفاف (transparent) بعدين لازم يبقى ADR جديد.

### على Kubernetes وgVisor
- **Kubernetes:** نفس الـ proxy شغّال في pod الـ runner. الـ agent بيوصّل `127.0.0.1:3128` بـ TCP لـ pod الـ runner، والـ NetworkPolicy بتمنع pods الـ jobs من أي حاجة تانية ([ADR-0048](0048-runner-packaging-and-kubernetes.md)).
- **gVisor:** نفس الـ unix socket، بـ `--host-uds=open` لـ `runsc` على مجلد التحكم بس.

## البدائل اللي اتدرست
- **pasta أو slirp4netns مع proxy:** pasta بيوصّل الحاوية لأي حتة الـ host يوصلها. ومنع الخروج المباشر كان هيحتاج قواعد nftables جوه الـ netns، وده رجوع للـ allowlist بالـ IP.
- **شبكة Podman داخلية (`--internal`) فيها container للـ proxy بشبكتين:** شغّال، لكنه بيضيف netavark وaardvark-dns (ولازم نتأكد إن الـ DNS مقفول فعلًا)، وcontainer زيادة لكل job، وحاجات أكتر ممكن تبوظ. ومفيش مكسب أمني على الـ unix socket.
- **nftables على IPs اتحلت من الأسماء:** بيكسر مع الـ CDNs، ومحتاج `CAP_NET_ADMIN` جوه الـ rootless netns.
- **MITM proxy بيفك TLS:** بيسمح بفلترة على مستوى الـ URL، لكنه محتاج CA جوه كل job، ومش ضروري للـ allowlist بالـ host.

## العواقب
- مفيش اعتمادية شبكة على الـ host: لا pasta ولا netavark ولا CNI.
- الـ proxy كود أمني في الـ runner، ومحتاج fuzz للـ parser بتاع القايمة وبتاع الـ CONNECT، واختبارات لكل نطاق ممنوع وللـ DNS rebinding.
- الـ jobs اللي محتاجة TCP مباشر لـ host معيّن من غير proxy مش هتشتغل لحد قرار جديد.
- سطور `system` في الـ log ممكن تكتر مع jobs كتير الاتصالات، فبتتجمّع: سطر واحد لكل host وقرار في الدقيقة.
