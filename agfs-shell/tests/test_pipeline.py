import unittest
from agfs_shell.pipeline import Pipeline
from agfs_shell.process import Process
from agfs_shell.streams import InputStream, OutputStream, ErrorStream

class TestPipeline(unittest.TestCase):
    def create_mock_process(self, name, output=None, exit_code=0):
        def executor(proc):
            if output:
                proc.stdout.write(output)
            # Read stdin to simulate consumption
            proc.stdin.read()
            return exit_code
            
        return Process(name, [], executor=executor)

    def create_echo_process(self, text):
        def executor(proc):
            proc.stdout.write(text)
            return 0
        return Process("echo", [text], executor=executor)

    def create_cat_process(self):
        def executor(proc):
            data = proc.stdin.read()
            proc.stdout.write(data)
            return 0
        return Process("cat", [], executor=executor)

    def test_single_process(self):
        p1 = self.create_mock_process("p1", output="hello", exit_code=0)
        pipeline = Pipeline([p1])
        
        self.assertEqual(pipeline.execute(), 0)
        self.assertEqual(pipeline.get_stdout(), b"hello")
        self.assertEqual(pipeline.get_exit_code(), 0)

    def test_pipeline_flow(self):
        # echo "hello" | cat
        p1 = self.create_echo_process("hello")
        p2 = self.create_cat_process()
        
        pipeline = Pipeline([p1, p2])
        
        self.assertEqual(pipeline.execute(), 0)
        self.assertEqual(pipeline.get_stdout(), b"hello")

    def test_pipeline_chain(self):
        # echo "hello" | cat | cat
        p1 = self.create_echo_process("hello")
        p2 = self.create_cat_process()
        p3 = self.create_cat_process()
        
        pipeline = Pipeline([p1, p2, p3])
        
        self.assertEqual(pipeline.execute(), 0)
        self.assertEqual(pipeline.get_stdout(), b"hello")

    def test_exit_code(self):
        # p1 (ok) | p2 (fail)
        p1 = self.create_mock_process("p1", exit_code=0)
        p2 = self.create_mock_process("p2", exit_code=1)
        
        pipeline = Pipeline([p1, p2])
        
        self.assertEqual(pipeline.execute(), 1)
        self.assertEqual(pipeline.get_exit_code(), 1)

    def test_empty_pipeline(self):
        pipeline = Pipeline([])
        self.assertEqual(pipeline.execute(), 0)
        self.assertEqual(pipeline.get_stdout(), b"")

if __name__ == '__main__':
    unittest.main()
