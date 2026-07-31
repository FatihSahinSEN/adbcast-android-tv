# ADBCast Chrome Extension - Chrome Web Store Submission Guide

This directory contains the Chrome Extension Manifest V3 source files ready for submission to the **Chrome Web Store Developer Dashboard**.

## 📌 Extension Details

- **Extension Name**: ADBCast - AOSP TV Bridge Connector
- **Version**: 1.1.0
- **Manifest Version**: 3
- **Developer / Author**: Fatih Şahin ŞEN ([fatihsahinsen@outlook.com](mailto:fatihsahinsen@outlook.com))
- **Official Website**: [IYC Yazılım (https://www.iyc.com.tr/)](https://www.iyc.com.tr/)

---

## 📦 How to Package for Chrome Web Store

1. Select all files inside the `extension/` directory (`manifest.json`, `popup.html`, `popup.js`, `icon.png`, `icon.svg`, screenshots).
2. Create a compressed `.zip` archive (e.g. `ADBCast-extension-v1.1.0.zip`).
3. Log in to the [Chrome Web Store Developer Dashboard](https://chrome.google.com/webstore/devconsole).
4. Click **Add new item** and upload `ADBCast-extension-v1.1.0.zip`.
5. Fill out store listing metadata:
   - **Category**: Productivity / Accessibility
   - **Privacy Policy**: Uses local ADB connection over localhost (`127.0.0.1:9468`). No user data is collected or sent to remote servers.
   - **Official Website**: `https://www.iyc.com.tr/`
   - **Support Email**: `fatihsahinsen@outlook.com`
6. Submit for review!
