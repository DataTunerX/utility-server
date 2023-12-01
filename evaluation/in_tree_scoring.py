import json
import os
import time
import requests
import random
import evaluate
from questions_references_en import questions_references_en_list
from questions_references_zh import questions_references_zh_list

inference_service = os.getenv("INFERENCE_SERVICE")
evaluation_language = os.getenv("EVALUATION_LANGUAGE")
complete_notify_url = os.getenv("COMPLETE_NOTIFY_URL")

if evaluation_language == "all":
    sample_size = 50
    random_indices = random.sample(range(len(questions_references_en_list)), sample_size)
    samples = (
        [questions_references_en_list[i] for i in random_indices] +
        [questions_references_zh_list[i] for i in random_indices]
    )
else:
    samples = (
        questions_references_en_list[:20]
        if evaluation_language == "en"
        else questions_references_zh_list
    )

questions = [sample['question'] for sample in samples]
predictions = []

max_retries = 3
retry_interval = 1

total_questions = len(questions)

for index, q in enumerate(questions, start=1):
    retries = 0
    while retries < max_retries:
        try:
            start_time = time.time()
            response = requests.post(
                inference_service,
                data=json.dumps({"input": q}),
                headers={"content-type": "application/json"}
            )
            response.raise_for_status()
            end_time = time.time()

            predictions.append(response.json()["output"])

            # 打印请求时间和进度
            print(f"Question {index}/{total_questions} | Request Time: {end_time - start_time:.2f} seconds")
            break
        except requests.RequestException as e:
            print(f"Error during request: {e}")
            retries += 1
            if retries < max_retries:
                print(f"Retrying in {retry_interval} seconds...")
                time.sleep(retry_interval)
            else:
                print(f"Max retries reached. Skipping question: {q}")

references = [sample['references'] for sample in samples]

rouge = evaluate.load("rouge")
rouge_result = rouge.compute(predictions=predictions, references=references)

bleu = evaluate.load("bleu")
bleu_result = bleu.compute(predictions=predictions, references=references)


rouge1_weight=0.35
rouge2_weight=0.4
rougeL_weight=0.15
rougeLsum_weight=0.1

rouge_score = (rouge1_weight*rouge_result['rouge1']*100 + \
              rouge2_weight*rouge_result['rouge2']*100 + \
              rougeL_weight*rouge_result['rougeL']*100 + \
              rougeLsum_weight*rouge_result['rougeLsum']*100)

bleu_score = bleu_result['bleu']*100

rouge_weight = 0.75
bleu_weight = 0.25

score = (rouge_weight*rouge_score + bleu_weight*bleu_score)


data = {"score": score, "metrics": ["ROUGE", "BLEU"], "details": {**rouge_result, **bleu_result}}
result = requests.post(complete_notify_url, data=json.dumps(data), headers={"content-type": "application/json"})
print(result.content)