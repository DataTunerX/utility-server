# Use the official Python image with version 3.10.12
FROM python:3.10.12

# Set the working directory in the container
WORKDIR /app

# Copy the current directory contents into the container at /app
COPY . /app

# Install the required dependencies
RUN pip install -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple

# Command to run your application
CMD ["python", "in_tree_scoring.py"]
