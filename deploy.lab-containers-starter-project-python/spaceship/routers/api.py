from fastapi import APIRouter
import numpy as np

router = APIRouter()


@router.get('')
def hello_world() -> dict:
    return {'msg': 'Hello, World!'}


@router.get('/matrix')
def matrix_multiply() -> dict:
    matrix_a = np.random.randint(0, 100, (10, 10))
    matrix_b = np.random.randint(0, 100, (10, 10))
    product = matrix_a @ matrix_b
    return {
        'matrix_a': matrix_a.tolist(),
        'matrix_b': matrix_b.tolist(),
        'product': product.tolist(),
    }
