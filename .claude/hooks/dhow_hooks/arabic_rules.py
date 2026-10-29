import posixpath
import re

from .common import GENERATED_PATH, is_test, new_lines

WEB_LOCALES = "apps/web/Src/I18n/Locales/"
GO_CATALOG_ROOTS = ("apps/server/internal/", "apps/runner/internal/", "apps/cli/")
GO_CATALOG_NAME = re.compile(r"(?:^|\.)[A-Za-z]*Ar\.go$")
EXCLUDED = ("apps/server/internal/Core/Installer/Prompt.go",)
ADR = "Plan/ADR/0062-simple-arabic-for-users.md"

DIACRITICS = re.compile(r"[ـً-ْٰ]")
ARABIC_LETTER = r"ء-يٱ-ۓ"
BEFORE = rf"(?<![{ARABIC_LETTER}A-Za-z0-9_])"
AFTER = rf"(?![{ARABIC_LETTER}A-Za-z0-9_])"
TAIL = rf"[{ARABIC_LETTER}]*"
CONJ = "[وف]?"

FUTURE_STEMS = (
    "ظهر", "خرج", "فضل", "قدر", "شوف", "قول", "بقى", "بقي", "حتاج", "عمل", "رجع", "دخل", "بطل", "كتب", "لاقي",
    "فتح", "طلب", "اخد", "بعت", "كمل", "نضم", "مسح", "شيل", "وقف", "جرب", "حفظ", "ستخدم", "غير", "لغي", "كون",
    "روح", "جيب", "نصب", "طبق", "شتغل", "سيب",
)
PROGRESSIVE_STEMS = (
    "شتغل", "بعت", "ظهر", "تأكد", "خلص", "قول", "عمل", "ستخدم", "تستخدم", "دير", "شوف", "حاول", "تفتح", "تسجل",
    "حتاج", "جهز", "رجع", "نتهي", "مسح", "تقفل", "تمسح", "تعمل", "تلغي", "تشال", "قفل", "كتب", "تبعت", "تحفظ",
    "تغير", "حصل", "سمع", "تنصب", "ساعد",
)
PROGRESSIVE_STEMS_NO_TA = ("حمل", "طلع", "دخل", "فضل")


def alternation(words):
    return "|".join(sorted(words, key=len, reverse=True))


RULES = (
    (rf"{CONJ}دلوقت(?:ي)?", "الآن"),
    (rf"{CONJ}(?:عايز|عاوز)(?:ة|ين)?", "تريد"),
    (rf"{CONJ}(?:إزاي|ازاي)", "كيف"),
    (rf"{CONJ}فين", "أين"),
    (rf"{CONJ}(?:كده|كدا)", "هكذا / بذلك، أو احذفها"),
    (rf"{CONJ}بت(?:ا|و)ع{TAIL}", "إضافة مباشرة (كلمة سرك) أو «الخاص بك»"),
    (rf"{CONJ}(?:مفيش|مافيش)", "لا يوجد / لا توجد"),
    (rf"{CONJ}(?:ب|ف)?اللي", "الذي / التي / مَن / ما"),
    (r"ده", "هذا"),
    (r"دي", "هذه"),
    (rf"{CONJ}مش", "لا / ليس / غير"),
    (rf"{CONJ}(?:ال)?تاني(?:ة|ين)?", "مرة أخرى / آخر / أخرى"),
    (rf"{CONJ}عشان", "لكي / حتى / لأن"),
    (rf"{CONJ}لسه", "بعد / ما زال / لم ... بعد"),
    (rf"{CONJ}مينفعش", "لا يمكن"),
    (rf"{CONJ}مقدرناش", "لم نتمكن من"),
    (rf"{CONJ}برضه", "أيضًا"),
    (r"(?:أيوه|ايوه)", "نعم"),
    (rf"{CONJ}ه[يتن](?:ت[{ARABIC_LETTER}]{{2,}}|(?:{alternation(FUTURE_STEMS)}){TAIL})",
     "المستقبل بـ «سـ»: «ستخرج» بدل «هتخرج»"),
    (rf"{CONJ}ب[يتن](?:{alternation(PROGRESSIVE_STEMS)}){TAIL}", "المضارع من غير الباء: «يعمل» بدل «بيشتغل»"),
    (rf"{CONJ}ب[ين](?:{alternation(PROGRESSIVE_STEMS_NO_TA)}){TAIL}", "المضارع من غير الباء: «يعمل» بدل «بيشتغل»"),
    (rf"{CONJ}ي?تعذر{TAIL}", "لا يمكن / لم نتمكن"),
    (rf"{CONJ}ريثما", "حتى / إلى أن"),
    (rf"{CONJ}آنفا", "سابقًا / من قبل"),
    (rf"{CONJ}لا ?سيما", "خاصةً"),
)
COMPILED = tuple((re.compile(BEFORE + "(?:" + pattern + ")" + AFTER), hint) for pattern, hint in RULES)


def is_arabic_catalog(rel):
    if rel in EXCLUDED or is_test(rel) or GENERATED_PATH.search(rel):
        return False
    name = posixpath.basename(rel)
    if rel.startswith(WEB_LOCALES):
        return name.endswith(".ts") and "Ar" in name
    if rel.startswith(GO_CATALOG_ROOTS):
        return GO_CATALOG_NAME.search(name) is not None
    return False


def find_marker(text):
    plain = DIACRITICS.sub("", text)
    for pattern, hint in COMPILED:
        found = pattern.search(plain)
        if found:
            return found.group(0), hint
    return None


def check_arabic_register(rel, added, old, **_):
    if not is_arabic_catalog(rel):
        return None
    for line in new_lines(added, old):
        hit = find_marker(line)
        if hit:
            word, hint = hit
            return (f"{rel}: «{word}» is not simple Arabic. User-facing Arabic is Modern Standard grammar with short "
                    f"sentences and common words, neither Egyptian colloquial nor heavy classical ({ADR}). "
                    f"Write: {hint}.")
    return None
