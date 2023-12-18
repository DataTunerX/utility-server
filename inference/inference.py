import re
import os
import time
from transformers import LlamaForCausalLM, LlamaTokenizer, GenerationConfig, pipeline
from peft import PeftModel
from ray import serve
from .model import ChatCompletionRequest, ChatCompletionResponse


origin_model_dir = os.getenv("BASE_MODEL_DIR")
checkpoint_dir = os.getenv("CHECKPOINT_DIR")

# Generate prompts from Llama2-13B template
def generate_prompt(input):
    return f"""
<s>[INST] <<SYS>>
You are a helpful, respectful and honest assistant. Always answer as helpfully as possible, while being safe.  Your answers should not include any harmful, unethical, racist, sexist, toxic, dangerous, or illegal content. Please ensure that your responses are socially unbiased and positive in nature.

If a question does not make any sense, or is not factually coherent, explain why instead of answering something not correct. If you don't know the answer to a question, please don't share false information.
<</SYS>>

{input} [/INST]
"""

class LlamaModel:
    def __init__(self):
        self.model = LlamaForCausalLM.from_pretrained(origin_model_dir)
        self.model = PeftModel.from_pretrained(self.model, checkpoint_dir).cuda().eval()
        self.tokenizer = LlamaTokenizer.from_pretrained(origin_model_dir)

    def generate(self, input, temperature: float = 0.1, top_p: float = 0.1, max_tokens: int = 10000, generation_kwargs={}):
        prompt = generate_prompt(input)
        inputs = self.tokenizer(prompt, return_tensors="pt")
        prompt_tokens = inputs["input_ids"].cuda().shape[1]
        config = GenerationConfig(
            do_sample=True,
            temperature=temperature,
            max_new_tokens=max_tokens,
            top_p=top_p,
            **generation_kwargs,
        )
        pipe = pipeline(
            "text-generation",
            model=self.model,
            tokenizer=self.tokenizer,
            batch_size=16, # TODO: make a parameter
            generation_config=config,
            device=0,
            framework="pt",
        )
        start_time = time.time()
        generated_text = pipe(prompt)[0]["generated_text"]
        end_time = time.time()
        inference_time = end_time - start_time
        # 使用正则表达式提取大模型的输出
        match = re.search(r'\[/INST\]\n(.+)$', generated_text, re.DOTALL)
        if match:
            model_output = match.group(1).strip()
            output = model_output
        else:
            output = ""
        completion_tokens = self.tokenizer(output, return_tensors="pt")
        choices = [{"index": 0, "message": {"role": "assistant", "content": output}, "logprobs": None, "finish_reason": "stop"}]
        usage = {
            "completion_tokens": str(completion_tokens), 
            "prompt_tokens": str(prompt_tokens), 
            "total_tokens": str(int(completion_tokens)+ int(prompt_tokens)), 
            "elasped_time": str(round(inference_time, 2)), 
            "token_per_sec": str(round((int(completion_tokens)+ int(prompt_tokens))/inference_time, 2))
        }
        resp = ChatCompletionResponse("id", choices, 0, "model", "system_fingerprint", usage)
        return resp.to_dict()


@serve.deployment(route_prefix="/chat/completions", ray_actor_options={"num_gpus": 1})
class LlamaDeployment:
    def __init__(self):
        self.model = LlamaModel()

    async def __call__(self, request):
        body = await request.json()
        input_data = body.get("messages").get("content")
        return self.model.generate(input_data)
    
deployment = LlamaDeployment.bind()