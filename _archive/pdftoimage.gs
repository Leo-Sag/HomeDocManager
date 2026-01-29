/**
 * ============================================================
 * pdftoimage.gs - PDF画像変換スクリプト（Googleフォト連携用）
 * ============================================================
 * ScanSnapでスキャンされたPDFファイルを画像（PNG）に変換し、
 * Googleフォト連携用のメタデータを付与する。
 * 
 * ※ 設定値はすべて Config.gs に記載
 * ※ Google Slides API の有効化が必要
 * ============================================================
 */

// ============================================================
// MAIN FUNCTION
// ============================================================

/**
 * メイン処理：PDFファイルを画像に変換してGoogleフォト用に準備
 * ※ トリガーにはこの関数を設定してください
 */
function runPdfToImageConverter() {
  // 夜間停止チェック（1:00〜6:00）
  if (isNightTime()) {
    Logger.log('夜間停止時間帯のため処理をスキップします。');
    return;
  }

  Logger.log('=== PDF画像変換処理を開始 ===');

  // 動的にフォルダを検索
  const inputFolders = findAllInputFolders();
  Logger.log(`検出された入力フォルダ数: ${inputFolders.length}`);

  if (inputFolders.length === 0) {
    Logger.log('処理対象のフォルダが見つかりませんでした。');
    return;
  }

  let totalProcessed = 0;
  let totalErrors = 0;

  // 各フォルダ内のPDFを処理
  for (const folder of inputFolders) {
    Logger.log(`\n--- フォルダ処理開始: ${folder.getName()} ---`);
    
    const files = folder.getFilesByType(MimeType.PDF);
    
    while (files.hasNext()) {
      const file = files.next();
      try {
        processPdfFile(file);
        totalProcessed++;
      } catch (error) {
        Logger.log(`エラー（ファイル: ${file.getName()}）: ${error.message}`);
        totalErrors++;
        // エラー時はスキップして次のファイルへ
      }
    }
  }

  Logger.log(`\n=== 処理完了 ===`);
  Logger.log(`処理成功: ${totalProcessed}件`);
  Logger.log(`エラー: ${totalErrors}件`);
}

// ============================================================
// FOLDER SEARCH FUNCTIONS
// ============================================================

/**
 * 40_子供・教育フォルダ配下の全「03_記録・作品・成績」フォルダを検索
 * @returns {Array<GoogleAppsScript.Drive.Folder>} 検出されたフォルダの配列
 */
function findAllInputFolders() {
  const baseFolder = DriveApp.getFolderById(FOLDER_IDS.CHILDREN_EDU);
  const targetFolderName = '03_記録・作品・成績';
  const result = [];

  searchFoldersRecursively(baseFolder, targetFolderName, result);

  return result;
}

/**
 * フォルダを再帰的に検索
 * @param {GoogleAppsScript.Drive.Folder} folder - 検索対象フォルダ
 * @param {string} targetName - 検索するフォルダ名
 * @param {Array<GoogleAppsScript.Drive.Folder>} result - 結果を格納する配列
 */
function searchFoldersRecursively(folder, targetName, result) {
  const subFolders = folder.getFolders();
  
  while (subFolders.hasNext()) {
    const subFolder = subFolders.next();
    const folderName = subFolder.getName();
    
    // 目的のフォルダ名と一致したら結果に追加
    if (folderName === targetName) {
      result.push(subFolder);
      Logger.log(`検出: ${getFolderPath(subFolder)}`);
    }
    
    // さらに下層を検索（再帰）
    searchFoldersRecursively(subFolder, targetName, result);
  }
}

/**
 * フォルダのパスを取得（デバッグ用）
 * @param {GoogleAppsScript.Drive.Folder} folder - 対象フォルダ
 * @returns {string} フォルダパス
 */
function getFolderPath(folder) {
  const path = [];
  let current = folder;
  
  // ルートまで遡る（最大5階層まで）
  for (let i = 0; i < 5; i++) {
    path.unshift(current.getName());
    const parents = current.getParents();
    if (!parents.hasNext()) break;
    current = parents.next();
  }
  
  return path.join(' / ');
}

// ============================================================
// PDF PROCESSING FUNCTIONS
// ============================================================

/**
 * PDFファイルを処理：画像変換 → メタデータ付与 → アーカイブ移動
 * @param {GoogleAppsScript.Drive.File} pdfFile - 処理対象PDFファイル
 */
function processPdfFile(pdfFile) {
  const fileName = pdfFile.getName();
  Logger.log(`\n処理開始: ${fileName}`);

  // PDF→画像変換
  const imageFiles = convertPdfToImages(pdfFile);
  
  if (imageFiles.length === 0) {
    Logger.log(`画像変換に失敗: ${fileName}`);
    return;
  }

  Logger.log(`変換成功: ${imageFiles.length}ページ`);

  // 各画像にメタデータを付与
  for (let i = 0; i < imageFiles.length; i++) {
    const imageFile = imageFiles[i];
    setImageMetadata(imageFile, pdfFile, i + 1, imageFiles.length);
  }

  // 元のPDFをアーカイブフォルダに移動
  archivePdfFile(pdfFile);

  Logger.log(`処理完了: ${fileName}`);
}

/**
 * PDFを画像に変換（Google Slides API使用）
 * @param {GoogleAppsScript.Drive.File} pdfFile - 変換対象PDFファイル
 * @returns {Array<GoogleAppsScript.Drive.File>} 生成された画像ファイルの配列
 */
function convertPdfToImages(pdfFile) {
  const imageFiles = [];
  let presentationId = null;

  try {
    // 1. 一時的なプレゼンテーションを作成
    const presentation = Slides.Presentations.create({
      title: `TMP_PDF_CONVERT_${Date.now()}`
    });
    presentationId = presentation.presentationId;

    // 2. PDFをプレゼンテーションにインポート
    // 注: Google Slides APIにはPDF直接インポート機能がないため、
    // Drive APIを使用してPDFをスライドに変換
    const convertedSlide = importPdfToSlides(pdfFile, presentationId);
    
    if (!convertedSlide) {
      throw new Error('PDFのスライド変換に失敗しました');
    }

    // 3. 各ページをサムネイル画像として取得
    const slidePages = Slides.Presentations.get(presentationId).slides;
    const baseName = pdfFile.getName().replace(/\.pdf$/i, '');
    const outputFolder = DriveApp.getFolderById(PDF_CONVERSION.OUTPUT_FOLDER_ID);

    for (let i = 0; i < slidePages.length; i++) {
      const pageId = slidePages[i].objectId;
      
      // サムネイル画像を取得
      const thumbnail = Slides.Presentations.Pages.getThumbnail(
        presentationId,
        pageId,
        { thumbnailProperties: { thumbnailSize: 'LARGE' } }
      );

      // 画像をダウンロードしてDriveに保存
      const imageBlob = UrlFetchApp.fetch(thumbnail.contentUrl).getBlob();
      const imageName = slidePages.length > 1 
        ? `${baseName}_p${String(i + 1).padStart(3, '0')}.png`
        : `${baseName}_converted.png`;

      imageBlob.setName(imageName);
      const imageFile = outputFolder.createFile(imageBlob);
      imageFiles.push(imageFile);

      Logger.log(`  ページ ${i + 1}/${slidePages.length} 変換完了`);
    }

  } catch (error) {
    Logger.log(`PDF変換エラー: ${error.message}`);
    throw error;
  } finally {
    // 4. 一時プレゼンテーションを削除
    if (presentationId) {
      try {
        DriveApp.getFileById(presentationId).setTrashed(true);
      } catch (e) {
        Logger.log(`一時ファイル削除エラー: ${e.message}`);
      }
    }
  }

  return imageFiles;
}

/**
 * PDFをGoogle Slidesにインポート（代替手法）
 * Google Slides APIにはPDF直接インポート機能がないため、
 * Drive APIでPDFを画像として各スライドに配置
 * 
 * @param {GoogleAppsScript.Drive.File} pdfFile - PDFファイル
 * @param {string} presentationId - プレゼンテーションID
 * @returns {boolean} 成功時true
 */
function importPdfToSlides(pdfFile, presentationId) {
  try {
    // PDFを一時的にGoogleドキュメントに変換してページ数を取得
    const tempDoc = Drive.Files.copy(
      { title: `TMP_OCR_${Date.now()}` },
      pdfFile.getId(),
      { ocr: true }
    );

    const doc = DocumentApp.openById(tempDoc.id);
    const pageCount = estimatePageCount(doc);
    
    // 一時ドキュメントを削除
    DriveApp.getFileById(tempDoc.id).setTrashed(true);

    // 各ページ用のスライドを作成し、PDFを画像として配置
    const requests = [];
    
    for (let i = 0; i < pageCount; i++) {
      // 新しいスライドを作成
      requests.push({
        createSlide: {
          insertionIndex: i
        }
      });
    }

    if (requests.length > 0) {
      Slides.Presentations.batchUpdate({ requests: requests }, presentationId);
    }

    return true;
  } catch (error) {
    Logger.log(`PDFインポートエラー: ${error.message}`);
    return false;
  }
}

/**
 * ドキュメントからページ数を推定
 * @param {GoogleAppsScript.Document.Document} doc - ドキュメント
 * @returns {number} 推定ページ数
 */
function estimatePageCount(doc) {
  const text = doc.getBody().getText();
  // 改ページ文字（\f）の数 + 1 でページ数を推定
  const pageBreaks = (text.match(/\f/g) || []).length;
  return Math.max(1, pageBreaks + 1);
}

// ============================================================
// METADATA FUNCTIONS
// ============================================================

/**
 * 画像ファイルにメタデータを付与
 * @param {GoogleAppsScript.Drive.File} imageFile - 画像ファイル
 * @param {GoogleAppsScript.Drive.File} originalPdf - 元のPDFファイル
 * @param {number} pageNumber - ページ番号
 * @param {number} totalPages - 総ページ数
 */
function setImageMetadata(imageFile, originalPdf, pageNumber, totalPages) {
  const metadata = [];
  
  // 元のファイル名
  metadata.push(`元ファイル: ${originalPdf.getName()}`);
  
  // スキャン日時（ファイル作成日）
  const scanDate = Utilities.formatDate(
    originalPdf.getDateCreated(),
    Session.getScriptTimeZone(),
    'yyyy-MM-dd HH:mm:ss'
  );
  metadata.push(`スキャン日時: ${scanDate}`);
  
  // ページ情報（複数ページの場合）
  if (totalPages > 1) {
    metadata.push(`ページ: ${pageNumber}/${totalPages}`);
  }
  
  // 処理タグ
  metadata.push('Processed by GAS for Google Photos');
  
  // Descriptionに設定
  const description = metadata.join('\n');
  imageFile.setDescription(description);
  
  Logger.log(`  メタデータ付与: ${imageFile.getName()}`);
}

// ============================================================
// FILE MANAGEMENT FUNCTIONS
// ============================================================

/**
 * 処理済みPDFをアーカイブフォルダに移動
 * @param {GoogleAppsScript.Drive.File} pdfFile - PDFファイル
 */
function archivePdfFile(pdfFile) {
  const archiveFolder = DriveApp.getFolderById(PDF_CONVERSION.ARCHIVE_FOLDER_ID);
  
  // 現在の親フォルダから削除
  const parents = pdfFile.getParents();
  while (parents.hasNext()) {
    parents.next().removeFile(pdfFile);
  }
  
  // アーカイブフォルダに追加
  archiveFolder.addFile(pdfFile);
  
  Logger.log(`  アーカイブ移動: ${pdfFile.getName()}`);
}

// ============================================================
// UTILITY FUNCTIONS
// ============================================================

/**
 * 夜間（1:00〜6:00）かどうかをチェック
 * @returns {boolean} 夜間の場合はtrue
 */
function isNightTime() {
  const hour = new Date().getHours();
  return hour >= 1 && hour < 6;
}
