
	resultChan := make(chan node.FlowNodeResult, 1)
	go nodeForEach.RunAsync(ctx, req, resultChan)

	// Wait for the initial result from RunAsync (indicates loop setup is done or immediate error)
	var result node.FlowNodeResult
	select {
	case result = <-resultChan:
		// Got the result from RunAsync
		if result.Err != nil {
			// Use Fatalf to stop the test immediately on error
			t.Fatalf("RunAsync returned an immediate error: %v", result.Err)
		}
	case <-time.After(1 * time.Second): // Short timeout for RunAsync to send its result
		t.Fatalf("Timed out waiting for RunAsync result channel")
	}

	// Now, wait for all async operations triggered by the loop to complete
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	// Wait with a timeout to prevent hanging tests
	select {
	case <-waitChan:
		// wg.Wait() completed successfully
	case <-time.After(5 * time.Second): // Adjust timeout as needed for async tasks
		t.Fatalf("Timed out waiting for WaitGroup (runCounter=%d)", runCounter.Load())
	}

	// Check the final count *after* waiting for the WaitGroup
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	}
