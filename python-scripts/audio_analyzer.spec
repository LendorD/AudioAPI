# audio_analyzer.spec
# -*- mode: python ; coding: utf-8 -*-

import glob
from PyInstaller.utils.hooks import collect_data_files

block_cipher = None

# Собираем все файлы Silero VAD рекурсивно
silero_datas = [(f, 'lib/silero-vad') for f in glob.glob('lib/silero-vad/**', recursive=True)]

# Опционально, если хочешь включить sklearn полностью
sklearn_datas = [(f, 'lib/sklearn') for f in glob.glob('lib/sklearn/**', recursive=True)]

# Если нужно включить torch/hubconf.py (Silero требует)
torch_datas = collect_data_files('torch')  # автоматически берет все файлы PyTorch

a = Analysis(
    ['script.py'],
    pathex=[],
    binaries=[
        ('C:/Users/dlucenko/Desktop/AudioAPI/AudioAPI/python-scripts/lib/vosk/libvosk.dll', 'vosk'),
    ],
    datas=[
        ('ffmpeg/ffmpeg.exe', 'ffmpeg'),
        ('models/vosk-model-ru-0.42/**', 'models/vosk-model-ru-0.42'),
    ] + silero_datas + sklearn_datas + torch_datas,
    hiddenimports=[
        'librosa',
        'scipy',
        'numpy',
        'vosk',
        'torch'
    ],
    hookspath=[],
    runtime_hooks=[],
    excludes=[],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name='audio_analyzer',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    console=True,
)

coll = COLLECT(
    exe,
    a.binaries,
    a.zipfiles,
    a.datas,
    strip=False,
    upx=True,
    upx_exclude=[],
    name='audio_analyzer'
)
