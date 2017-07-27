import logging

logging.basicConfig(level=logging.DEBUG,
                    format='%(asctime)s %(levelname)s %(message)s')


def critical(msg, *args, **kwargs):
    logging.critical("\x1b[0;31m " + msg + "\x1b[0m", *args, **kwargs)
    exit(1)


def error(msg, *args, **kwargs):
    logging.error("\x1b[0;31m " + msg + "\x1b[0m", *args, **kwargs)


def warn(msg, *args, **kwargs):
    logging.warn("\x1b[0;33m " + msg + "\x1b[0m", *args, **kwargs)


def info(msg, *args, **kwargs):
    logging.info(msg, *args, **kwargs)
