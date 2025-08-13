import io
import os
import tempfile
import numpy as np
import librosa
import torch
from vosk import Model, KaldiRecognizer
import json
import wave
import subprocess
from pathlib import Path
import sys
from typing import List, Dict, Any
from scipy.cluster.hierarchy import fcluster, linkage  # Альтернатива для кластеризации

# утсановка кодировки
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

# Конфигурация

VOSK_MODEL_PATH = "" 
SILERO_VAD_PATH = ""
# Получаем путь к папке с exe
if getattr(sys, 'frozen', False):
    # Если запущен как exe
    base_dir = os.path.dirname(sys.executable)
    # Для PyInstaller нужно добавить модель в datas
    VOSK_MODEL_PATH = os.path.join(base_dir, "vosk-model-ru-0.42")
 
else:
    # Если запущен как скрипт
    base_dir = os.path.dirname(os.path.abspath(__file__))
    VOSK_MODEL_PATH = os.path.join(base_dir, "models", "vosk-model-ru-0.42")
    SILERO_VAD_PATH = os.path.join(base_dir, "lib", "silero-vad")

SAMPLE_RATE = 16000
FFMPEG_PATH = os.path.join(base_dir, "ffmpeg", "ffmpeg.exe")

# Инициализация моделей
def load_models():
    global SILERO_VAD_PATH, VOSK_MODEL_PATH  # Указываем, что используем глобальную переменную
    # 1. Инициализация Silero VAD
    vad_model, utils = torch.hub.load(
        repo_or_dir=SILERO_VAD_PATH,
        model='silero_vad',
        source='local',
        force_reload=True,
        trust_repo=True
    )
    

    # 2. Инициализация Vosk
    if not os.path.exists(VOSK_MODEL_PATH):
        # Попробуем найти модель во временной папке PyInstaller
        if getattr(sys, 'frozen', False):
            base_path = sys._MEIPASS
            VOSK_MODEL_PATH = os.path.join(base_path, "vosk-model-ru-0.42")
        
        if not os.path.exists(VOSK_MODEL_PATH):
            raise FileNotFoundError(f"Модель Vosk не найдена по пути {VOSK_MODEL_PATH}")
    
    vosk_model = Model(VOSK_MODEL_PATH)
    
    return vad_model, utils, vosk_model

# Надежная конвертация аудио
def convert_audio(input_path: str, output_path: str):
    """Конвертирует аудио в WAV 16kHz mono"""
    try:
        subprocess.run(
            [FFMPEG_PATH, "-i", input_path, "-ar", str(SAMPLE_RATE), "-ac", "1", output_path, "-y"],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            creationflags=subprocess.CREATE_NO_WINDOW
        )
        # Проверяем, что файл создан
        if not os.path.exists(output_path):
            raise RuntimeError("Файл не был создан после конвертации")
    except subprocess.CalledProcessError as e:
        error_msg = e.stderr.decode('utf-8', errors='ignore') if e.stderr else str(e)
        raise RuntimeError(f"Ошибка конвертации: {error_msg}")

# Безопасная работа с временными файлами
class TempFileManager:
    def __init__(self, suffix=""):
        self.path = None
        self.suffix = suffix
    
    def __enter__(self):
        fd, self.path = tempfile.mkstemp(suffix=self.suffix)
        os.close(fd)  # Закрываем дескриптор, чтобы не блокировать файл
        return self.path
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.path and os.path.exists(self.path):
            try:
                os.unlink(self.path)
            except:
                try:
                    os.chmod(self.path, 0o777)
                    os.unlink(self.path)
                except:
                    pass

def analyze_audio_file(audio_path: str, num_speakers: int = 2, vad_threshold: float = 0.5) -> List[Dict[str, Any]]:
    """
    Анализирует аудиофайл и возвращает сегменты с текстом и идентификатором говорящего
    
    :param audio_path: Путь к аудиофайлу
    :param num_speakers: Количество говорящих
    :param vad_threshold: Порог детекции речи (0-1)
    :return: Список сегментов с текстом
    """
    with TempFileManager(suffix=".wav") as temp_wav:
        try:
            # 1. Конвертируем в WAV
            convert_audio(audio_path, temp_wav)

            # 2. Проверяем WAV-файл перед обработкой
            try:
                with wave.open(temp_wav, "rb") as wf:
                    if wf.getnchannels() != 1 or wf.getsampwidth() != 2:
                        raise RuntimeError("Аудио должно быть в формате WAV mono 16bit")
            except wave.Error as e:
                raise RuntimeError(f"Неверный WAV-файл: {str(e)}")

            # 3. Детекция речи (VAD)
            audio = read_audio(temp_wav, sampling_rate=SAMPLE_RATE)
            speech_segments = get_speech_timestamps(audio, vad_model, threshold=vad_threshold)

            # 4. Извлечение признаков (MFCC)
            def extract_features(segment):
                y, sr = librosa.load(temp_wav, sr=SAMPLE_RATE,
                                offset=segment['start']/SAMPLE_RATE,
                                duration=(segment['end']-segment['start'])/SAMPLE_RATE)
                mfcc = librosa.feature.mfcc(y=y, sr=sr, n_mfcc=13)
                return np.mean(mfcc, axis=1)

            features = np.array([extract_features(seg) for seg in speech_segments])

            # 5. Кластеризация (измененная часть)
            if len(features) > 1:
                # Используем иерархическую кластеризацию из scipy
                Z = linkage(features, method='ward')
                speaker_labels = fcluster(Z, t=num_speakers, criterion='maxclust') - 1  # Приводим к 0-based индексации
            else:
                speaker_labels = [0] * len(speech_segments)

            # 6. Транскрибация Vosk
            wf = wave.open(temp_wav, "rb")
            recognizer = KaldiRecognizer(vosk_model, wf.getframerate())
            recognizer.SetWords(True)

            results = []
            while True:
                data = wf.readframes(4000)
                if len(data) == 0:
                    break
                if recognizer.AcceptWaveform(data):
                    results.append(json.loads(recognizer.Result()))
            
            results.append(json.loads(recognizer.FinalResult()))
            wf.close()

            # 7. Формирование результата
            output = []
            for seg, label in zip(speech_segments, speaker_labels):
                start = seg['start'] / SAMPLE_RATE
                end = seg['end'] / SAMPLE_RATE
                
                words = []
                for res in results:
                    if 'result' not in res:
                        continue
                    for word in res['result']:
                        if start <= word['start'] <= end:
                            words.append(word['word'])
                
                output.append({
                    "start": round(start, 2),
                    "end": round(end, 2),
                    "speaker": f"SPEAKER_{label}",
                    "text": " ".join(words).strip()
                })

            return output

        except Exception as e:
            print(f"Ошибка обработки: {str(e)}", file=sys.stderr)
            return []

def main():
    # Инициализация моделей при старте
    global vad_model, vad_utils, vosk_model, get_speech_timestamps, read_audio, VOSK_MODEL_PATH, SILERO_VAD_PATH
    
    try:
        vad_model, vad_utils, vosk_model = load_models()
        (get_speech_timestamps, _, read_audio, _, _) = vad_utils
    except Exception as e:
        print(f"FATAL ERROR: {str(e)}", file=sys.stderr)
        sys.exit(1)

    if len(sys.argv) < 2:
        print("Использование: python audio_analyzer.py <путь_к_аудиофайлу> [количество_говорящих=2] [порог_VAD=0.5]")
        sys.exit(1)

    audio_path = sys.argv[1]
    num_speakers = int(sys.argv[2]) if len(sys.argv) > 2 else 2
    vad_threshold = float(sys.argv[3]) if len(sys.argv) > 3 else 0.5

    if not os.path.exists(audio_path):
        print(f"Ошибка: файл '{audio_path}' не найден", file=sys.stderr)
        sys.exit(1)

    segments = analyze_audio_file(audio_path, num_speakers, vad_threshold)

    # Вывод в JSON (с отступами для читаемости)
    print("-START-")
    # print(segments)
    print(json.dumps(segments, indent=2, ensure_ascii=False))

if __name__ == "__main__":
    main()