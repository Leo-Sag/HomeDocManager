/**
 * ============================================================
 * Config.gs - 設定ファイル
 * ============================================================
 * すべての設定値をここに集約。
 * FileSorter.gs と CalendarSync.gs から参照される。
 * ============================================================
 */

// ============================================================
// CONFIGURATION SECTION - ユーザー設定（必ず書き換えてください）
// ============================================================

/** Gemini API Key（Google AI Studioで取得） */
const GEMINI_API_KEY = 'AIzaSyCpsaJ6425l0NbgjdPaZFIdA3kchwma1ZU';

/** フォルダID設定 */
const FOLDER_IDS = {
  SOURCE: '1T_XJURJbSsSiarr2Y-ofH0lCpSn9Dmak',              // 00_Inbox
  MONEY_TAX: '1rUnmoPoJoD-UwLn0PQW7-FtBfg9FlUTi',           // 10_マネー・税務
  PROJECT_ASSET: '1xBNSHmmnpuQpz0pvXxg_VlUAy0Zk4SOG',       // 20_プロジェクト・資産
  LIFE_ADMIN: '1keZdfSSrmpPqPWhC22Fg2A5GmaCfg3Xg',          // 30_ライフ・行政
  CHILDREN_EDU: '14TyZrKoXRSSP6kxpytxvap4poKmDn4qs',        // 40_子供・教育
  PHOTO_OTHER: '1euBhhNI0Ny13tXs1JVrcO0KLKHySFnEy',         // 50_写真・その他
  LIBRARY: '1MxppChMYZOJOyY2s-w6CsVam3P5_vccv',                             // 90_ライブラリ
  NOTEBOOKLM_SYNC: '1AVRbK5Zy8IVC3XYtSQ7ZwNGMIB3ToaBu',     // 95_NotebookLM_Sync
  ARCHIVE: '14iqjkHeBVMz47sNzPFkxrp5syr2tIOeO'                              // 99_転送済みアーカイブ
};

/** CalendarSync.gsで使用する40_子供・教育フォルダIDのエイリアス */
const CHILDREN_EDU_FOLDER_ID = FOLDER_IDS.CHILDREN_EDU;

/** カテゴリ名とフォルダIDのマッピング */
const CATEGORY_MAP = {
  '10_マネー・税務': FOLDER_IDS.MONEY_TAX,
  '20_プロジェクト・資産': FOLDER_IDS.PROJECT_ASSET,
  '30_ライフ・行政': FOLDER_IDS.LIFE_ADMIN,
  '40_子供・教育': FOLDER_IDS.CHILDREN_EDU,
  '50_写真・その他': FOLDER_IDS.PHOTO_OTHER,
  '90_ライブラリ': FOLDER_IDS.LIBRARY,
  '99_転送済みアーカイブ': FOLDER_IDS.ARCHIVE
};

/** お子様リストと名寄せルール（検索キーワード → 正規フォルダ名） */
const CHILD_ALIASES = {
  '明日香': ['明日香', 'あすか', 'アスカ', 'Asuka'],
  '遥香': ['遥香', 'はるか', 'ハルカ', 'Haruka'],
  '文香': ['文香', 'ふみか', 'フミカ', 'Fumika'],
  'ビクトル': ['ビクトル', 'Victor', 'Viktor'],
  'ミハイル': ['ミハイル', 'Mikhail', 'Mihail'],
  'アンナ': ['アンナ', 'Anna']
};

/** サブカテゴリ一覧（40_子供・教育用） */
const SUB_CATEGORIES = [
  '01_お便り・スケジュール',
  '02_提出・手続き・重要',
  '03_記録・作品・成績'
];

/**
 * 監視対象のサブフォルダ名（CalendarSync用）
 * ※ 01_お便り・スケジュール と 02_提出・手続き・重要 の両方を対象とする
 */
const TARGET_SUBFOLDER_NAMES = [
  '01_お便り・スケジュール',
  '02_提出・手続き・重要'
];

/** 登録先カレンダーID（プライマリーカレンダーは 'primary' を指定） */
const CALENDAR_ID = '639243bb722810f6fbe8f95b9dc57adf65677a53810d7fcdc76eef0fc4845792@group.calendar.google.com';

/** PDF画像変換用フォルダID設定 */
const PDF_CONVERSION = {
  // 出力フォルダ（n8n監視用・共通）
  OUTPUT_FOLDER_ID: 'YOUR_OUTPUT_FOLDER_ID',
  // アーカイブフォルダ（処理済みPDF保管用）
  ARCHIVE_FOLDER_ID: 'YOUR_ARCHIVE_FOLDER_ID'
  // 注: 入力フォルダは動的検索（40_子供・教育配下の全03_記録・作品・成績フォルダ）
};
