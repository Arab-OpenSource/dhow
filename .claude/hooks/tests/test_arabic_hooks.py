import os
import sys
import unittest

HOOKS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, HOOKS)

from dhow_hooks import arabic_rules, edit_rules

ROOT = os.path.dirname(os.path.dirname(HOOKS))

WEB_AR = "apps/web/Src/I18n/Locales/Workspace/JobsAr.ts"
SERVER_AR = "apps/server/internal/Core/I18n/I18n.Ar.go"
INSTALLER_AR = "apps/server/internal/Core/Installer/Messages.Ar.go"


def edit(rel, added, old=""):
    return edit_rules.evaluate(rel=rel, root="/nonexistent", segments=[(added, old)], exists=False, final_content=added)


def ts_line(text):
    return "export const X = {\n  'a.b': '" + text + "',\n} as const;\n"


def go_line(text):
    return 'package i18n\n\nvar arMessages = map[string]string{\n\t"a.b": "' + text + '",\n}\n'


COLLOQUIAL = (
    ("الحالة دلوقتي", "دلوقتي", "الآن"),
    ("الحالة دلوقت", "دلوقت", "الآن"),
    ("لو عايز تكمّل", "عايز", "تريد"),
    ("لو عاوز تكمّل", "عاوز", "تريد"),
    ("إزاي تضيف جهاز", "إزاي", "كيف"),
    ("ازاي تضيف جهاز", "ازاي", "كيف"),
    ("الملف فين", "فين", "أين"),
    ("كده هتخرج", "كده", "هكذا"),
    ("وكدا خلاص", "وكدا", "هكذا"),
    ("الحساب بتاع المؤسسة", "بتاع", "الخاص بك"),
    ("الصفحة بتاعة الإعدادات", "بتاعة", "الخاص بك"),
    ("كلمة السر بتاعتك", "بتاعتك", "الخاص بك"),
    ("الإعدادات بتوعك", "بتوعك", "الخاص بك"),
    ("مفيش أجهزة", "مفيش", "لا يوجد"),
    ("مافيش أجهزة", "مافيش", "لا يوجد"),
    ("الجهاز اللي اخترته", "اللي", "الذي"),
    ("كل اللي في الطابور", "اللي", "الذي"),
    ("واللي بعده", "واللي", "الذي"),
    ("الصفحة دي", "دي", "هذه"),
    ("الجهاز ده", "ده", "هذا"),
    ("الرابط مش شغّال", "مش", "ليس"),
    ("ومش هيظهر", "ومش", "ليس"),
    ("جرّب تاني", "تاني", "مرة أخرى"),
    ("الأجهزة التانية", "التانية", "مرة أخرى"),
    ("عشان تكمّل", "عشان", "لكي"),
    ("لسه ماخلصش", "لسه", "ما زال"),
    ("مينفعش تتشال", "مينفعش", "لا يمكن"),
    ("مقدرناش نحمّل", "مقدرناش", "لم نتمكن"),
    ("ومعاه برضه", "برضه", "أيضًا"),
    ("أيوه، الغيه", "أيوه", "نعم"),
    ("الرابط هيظهر هنا", "هيظهر", "سـ"),
    ("هتخرج من كل الأجهزة", "هتخرج", "سـ"),
    ("هنقدر نكمّل", "هنقدر", "سـ"),
    ("الأكواد هتبطل تشتغل", "هتبطل", "سـ"),
    ("الدعوة هتتبعت", "هتتبعت", "سـ"),
    ("هيتعمل رابط جديد", "هيتعمل", "سـ"),
    ("هنتأكد منها", "هنتأكد", "سـ"),
    ("السيرفر بيشتغل", "بيشتغل", "يعمل"),
    ("الواجهة بتشتغل", "بتشتغل", "يعمل"),
    ("بنحمّل الأعضاء", "بنحمل", "يعمل"),
    ("الرابط بيتستخدم مرة واحدة", "بيتستخدم", "يعمل"),
    ("بنتأكد من الهوية", "بنتأكد", "يعمل"),
    ("الجلسة بتنتهي بعد ساعة", "بتنتهي", "يعمل"),
    ("بيطلّع السطر", "بيطلع", "يعمل"),
)

HEAVY = (
    ("يتعذّر الاتصال", "يتعذر", "لا يمكن"),
    ("تعذّرت قراءة الملف", "تعذرت", "لا يمكن"),
    ("انتظر ريثما ينتهي", "ريثما", "حتى"),
    ("كما ذكرنا آنفًا", "آنفا", "سابقًا"),
    ("لاسيما في الإنتاج", "لاسيما", "خاصةً"),
    ("لا سيما في الإنتاج", "لا سيما", "خاصةً"),
)

STANDARD = (
    "لم نتمكن من تحميل الأعضاء. حاول مرة أخرى.",
    "هنا يظهر رابط الدعوة.",
    "هذا الجهاز وهذه الصفحة.",
    "هي التي تحدد الوسوم، وهو الذي يرسلها.",
    "هيئة البيانات وهيكل الجدول.",
    "هناك جهاز آخر، وهنا جهاز ثالث.",
    "هتاف الجمهور والهندسة.",
    "بيان الحساب، وبيانات المؤسسة، والبيع، والبيئة المعزولة.",
    "بين الأعضاء وبيت الخبرة.",
    "أرسلناه بالبريد الإلكتروني بتاريخ اليوم.",
    "بتعديل الوسوم وبتحميل الصورة وبتسجيل الجهاز وبتجهيز البيئة.",
    "بتحمّل المسؤولية وبتدخّل المدير.",
    "بنجاح، وبنفسك، وبنسبة كبيرة، وبنقل الملكية.",
    "بنشر النسخة وبنظام واضح.",
    "فينا من يعرف، والمدينة والبدين.",
    "الثاني والثانية وتانيت.",
    "مشى ومشروع ومشكلة ومشاهد ومشغّل.",
    "بعد ما ودّه وودّي.",
    "هندسة وهنود وهيمنة.",
    "قدير ومقدار وتدير.",
    "دين ودينار وديسمبر.",
    "سيما وسيماء.",
    "الأنف والآن.",
)


class ArabicRegisterRules(unittest.TestCase):
    def test_each_colloquial_marker_is_rejected_with_its_replacement(self):
        for text, word, hint in COLLOQUIAL:
            for rel, body in ((WEB_AR, ts_line(text)), (SERVER_AR, go_line(text)), (INSTALLER_AR, go_line(text))):
                reason = edit(rel, body)
                self.assertIsNotNone(reason, f"{rel}: {text}")
                self.assertIn(f"«{word}»", reason, text)
                self.assertIn(hint, reason, text)
                self.assertIn("ADR/0062", reason, text)

    def test_each_heavy_classical_marker_is_rejected(self):
        for text, word, hint in HEAVY:
            reason = edit(WEB_AR, ts_line(text))
            self.assertIsNotNone(reason, text)
            self.assertIn(f"«{word}»", reason, text)
            self.assertIn(hint, reason, text)

    def test_markers_with_diacritics_or_tatweel_are_still_caught(self):
        for text in ("دلوقـتي", "مِش", "اللّي", "يتعذَّر"):
            self.assertIsNotNone(edit(WEB_AR, ts_line(text)), text)

    def test_standard_arabic_look_alikes_pass(self):
        for text in STANDARD:
            self.assertIsNone(edit(WEB_AR, ts_line(text)), text)
            self.assertIsNone(edit(SERVER_AR, go_line(text)), text)

    def test_every_arabic_catalog_is_checked(self):
        for rel in ("apps/web/Src/I18n/Locales/Ar.ts", "apps/web/Src/I18n/Locales/Boot/Ar.ts",
                    "apps/web/Src/I18n/Locales/Workspace/Ar.ts", "apps/web/Src/I18n/Locales/Workspace/HomeAr.ts",
                    "apps/web/Src/I18n/Locales/Workspace/OrgPagesAr.ts", SERVER_AR, INSTALLER_AR,
                    "apps/server/internal/Core/Mail/Templates.Ar.go", "apps/runner/internal/Logs/MessagesAr.go"):
            body = ts_line("دلوقتي") if rel.endswith(".ts") else go_line("دلوقتي")
            self.assertIn("ADR/0062", edit(rel, body) or "", rel)

    def test_excluded_files_pass(self):
        for rel in ("apps/server/internal/Core/Installer/Prompt.go", "apps/server/internal/Core/Installer/Prompt_test.go",
                    "apps/server/internal/Core/I18n/I18n_test.go", "apps/web/Src/I18n/I18n.test.tsx",
                    "apps/web/Src/I18n/Locales/Workspace/JobsEn.ts", "apps/server/internal/Core/I18n/I18n.En.go",
                    "apps/server/internal/Core/Installer/Answers.go"):
            body = go_line("أيوه، مش دلوقتي") if rel.endswith(".go") else ts_line("أيوه، مش دلوقتي")
            reason = edit(rel, body)
            self.assertNotIn("ADR/0062", reason or "", rel)
        for rel in ("Plan/Progress/2026-10-29.md", ".claude/hooks/README.md", "Plan/ADR/0062-simple-arabic-for-users.md"):
            self.assertIsNone(edit(rel, "عايز إيه دلوقتي؟ مفيش حاجة.\n"), rel)

    def test_only_new_lines_are_checked(self):
        old = ts_line("جرّب تاني")
        added = old.replace("} as const;", "  'c.d': 'حاول مرة أخرى',\n} as const;")
        self.assertIsNone(edit(WEB_AR, added, old))


class CurrentCatalogs(unittest.TestCase):
    def catalogs(self):
        found = []
        for base in ("apps/web/Src/I18n/Locales", "apps/server/internal", "apps/runner/internal", "apps/cli"):
            for folder, _, names in os.walk(os.path.join(ROOT, base)):
                for name in names:
                    rel = os.path.relpath(os.path.join(folder, name), ROOT).replace(os.sep, "/")
                    if arabic_rules.is_arabic_catalog(rel):
                        found.append(rel)
        return sorted(found)

    def test_every_arabic_catalog_in_the_repository_is_simple_arabic(self):
        found = self.catalogs()
        for rel in ("apps/web/Src/I18n/Locales/Boot/Ar.ts", "apps/web/Src/I18n/Locales/Workspace/Ar.ts", SERVER_AR,
                    INSTALLER_AR):
            self.assertIn(rel, found)
        for rel in found:
            with open(os.path.join(ROOT, rel), encoding="utf-8") as fh:
                self.assertIsNone(arabic_rules.check_arabic_register(rel=rel, added=fh.read(), old=""), rel)


if __name__ == "__main__":
    unittest.main()
