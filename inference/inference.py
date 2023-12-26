import re
import os
import time
from transformers import LlamaForCausalLM, LlamaTokenizer, GenerationConfig, pipeline
from peft import PeftModel
from ray import serve


class ChatCompletionRequest:
    def __init__(self, messages, model, frequency_penalty=0, logit_bias=None, max_tokens=None, n=1,
                 presence_penalty=0, response_format={"type": "json_object"}, seed=None, stop=None,
                 stream=False, temperature=1, top_p=1, tools=None, tool_choice="auto", user=None):
        self.messages = [ChatCompletionMessage.from_dict(message_data) for message_data in messages]
        self.model = model
        self.frequency_penalty = frequency_penalty
        self.logit_bias = logit_bias
        self.max_tokens = max_tokens
        self.n = n
        self.presence_penalty = presence_penalty
        self.response_format = ChatCompletionResponseFormat.from_dict(response_format)
        self.seed = seed
        self.stop = stop
        self.stream = stream
        self.temperature = temperature
        self.top_p = top_p
        self.tools = [ChatCompletionTool.from_dict(tool_data) for tool_data in tools] if tools else None
        self.tool_choice = ChatCompletionToolChoice.from_dict(tool_choice)
        self.user = user

    def to_dict(self):
        return {
            "messages": [message.to_dict() for message in self.messages],
            "model": self.model,
            "frequency_penalty": self.frequency_penalty,
            "logit_bias": self.logit_bias,
            "max_tokens": self.max_tokens,
            "n": self.n,
            "presence_penalty": self.presence_penalty,
            "response_format": self.response_format.to_dict(),
            "seed": self.seed,
            "stop": self.stop,
            "stream": self.stream,
            "temperature": self.temperature,
            "top_p": self.top_p,
            "tools": [tool.to_dict() for tool in self.tools] if self.tools else None,
            "tool_choice": self.tool_choice.to_dict(),
            "user": self.user,
        }

    @classmethod
    def from_json(cls, json_data):
        # 使用传入的 JSON 数据创建对象
        return cls(**json_data)

    def validate(self):
        # 验证 model 字段
        if not self.model or not isinstance(self.model, str):
            raise ValueError("Invalid model ID. Model ID must be a non-empty string.")

        # 验证 messages 字段
        if not self.messages or not isinstance(self.messages, list):
            raise ValueError("Invalid messages field. It must be a non-empty list of message objects.")

        for message in self.messages:
            if not isinstance(message, dict) or "role" not in message or "content" not in message:
                raise ValueError("Invalid message format. Each message must be a dictionary with 'role' and 'content'.")

            if message["role"] not in ["system", "user", "assistant"]:
                raise ValueError("Invalid role. Role must be one of 'system', 'user', or 'assistant'.")

            if not isinstance(message["content"], str) or not message["content"]:
                raise ValueError("Invalid message content. It must be a non-empty string.")


class ChatCompletionMessage:
    def __init__(self, content, role):
        self.content = content
        self.role = role

    def to_dict(self):
        return {"content": self.content, "role": self.role}

    @classmethod
    def from_dict(cls, message_data):
        return cls(**message_data)


class ChatCompletionResponseFormat:
    def __init__(self, type):
        self.type = type

    def to_dict(self):
        return {"type": self.type}

    @classmethod
    def from_dict(cls, response_format_data):
        return cls(**response_format_data)


class ChatCompletionTool:
    def __init__(self, name):
        self.name = name

    def to_dict(self):
        return {"name": self.name}

    @classmethod
    def from_dict(cls, tool_data):
        return cls(**tool_data)


class ChatCompletionToolChoice:
    def __init__(self, type):
        self.type = type

    def to_dict(self):
        return {"type": self.type}

    @classmethod
    def from_dict(cls, tool_choice_data):
        return cls(**tool_choice_data)



class ChatCompletionChoice:
    def __init__(self, content, role, tool_calls=None):
        self.content = content
        self.role = role
        self.tool_calls = tool_calls or []

    def to_dict(self):
        return {
            "content": self.content,
            "role": self.role,
            "tool_calls": self.tool_calls,
        }

    @classmethod
    def from_dict(cls, choice_data):
        return cls(**choice_data)

class ChatCompletionResponse:
    def __init__(self, id, choices, created, model, system_fingerprint, usage):
        self.id = id
        self.choices = choices
        self.created = created
        self.model = model
        self.system_fingerprint = system_fingerprint
        self.object = "chat.completion"
        self.usage = usage

    def to_dict(self):
        return {
            "id": self.id,
            "choices": self.choices,
            "created": self.created,
            "model": self.model,
            "system_fingerprint": self.system_fingerprint,
            "object": self.object,
            "usage": self.usage,
        }

    @classmethod
    def from_dict(cls, response_data):
        choices_data = response_data.get("choices", [])
        choices = [ChatCompletionChoice.from_dict(choice_data) for choice_data in choices_data]
        return cls(choices=choices, **response_data)

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
        completion_tokens = self.tokenizer(output, return_tensors="pt")["input_ids"].cuda().shape[1]
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
        input_data = body.get("messages")[0].get("content")
        temperature = body.get("temperature", 0.1)
        top_p = body.get("top_p", 0.1)
        return self.model.generate(input_data, temperature, top_p)
    
deployment = LlamaDeployment.bind()