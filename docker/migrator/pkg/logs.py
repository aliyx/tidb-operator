import logging

logging.basicConfig(level=logging.DEBUG,
                    format='%(asctime)s %(levelname)s %(message)s')

def critical(msg, *args, **kwargs):
  logging.critical(msg, *args, **kwargs)
  exit(1)

def error(msg, *args, **kwargs):
  logging.error(msg, *args, **kwargs)

def info(msg, *args, **kwargs):
  logging.info(msg, *args, **kwargs)