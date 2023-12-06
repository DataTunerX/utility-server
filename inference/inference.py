import re
import os
from transformers import LlamaForCausalLM, LlamaTokenizer, GenerationConfig, pipeline
from peft import PeftModel
from ray import serve
import asyncio

# origin_model_dir = "/data/llms/llama2-7b"
# checkpoint_dir = "/ray_results/TorchTrainer_73434_00000_0_2023-11-22_23-26-10/result"

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
        input_ids = inputs["input_ids"].cuda()  # 将输入移到 GPU 上
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
        generated_text = pipe(prompt)[0]["generated_text"]
        # 使用正则表达式提取大模型的输出
        match = re.search(r'\[/INST\]\n(.+)$', generated_text, re.DOTALL)
        if match:
            model_output = match.group(1).strip()
            output = model_output
        else:
            output = ""
        return {"output": output}


@serve.deployment(route_prefix="/inference", ray_actor_options={"num_gpus": 1})
class LlamaDeployment:
    def __init__(self):
        self.model = LlamaModel()

    async def __call__(self, request):
        body = await request.json()
        input_data = body.get("input")
        task = asyncio.create_task(self.model.generate(input_data))
        result = await task
        return result
    
deployment = LlamaDeployment.bind()